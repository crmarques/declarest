// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promptauth

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
)

func TestRuntimeResolvePromptsOnceAndReusesSameCredential(t *testing.T) {
	isolatePromptAuthEnv(t)

	store := &memorySessionStore{}
	prompter := &stubPrompter{
		values: map[string]string{
			"shared.username": "shared-user",
			"shared.password": "shared-pass",
		},
	}

	runtime, err := New(
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	first, err := runtime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(first) returned error: %v", err)
	}
	second, err := runtime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(second) returned error: %v", err)
	}

	if first != second {
		t.Fatalf("expected reused credentials, got %#v and %#v", first, second)
	}
	if prompter.promptCalls != 2 {
		t.Fatalf("expected two field prompts, got %d", prompter.promptCalls)
	}
}

func TestRuntimeResolveKeepsPromptedValuesForSession(t *testing.T) {
	isolatePromptAuthEnv(t)

	store := &memorySessionStore{}
	prompter := &stubPrompter{
		values: map[string]string{
			"kept.username": "kept-user",
			"kept.password": "kept-pass",
		},
	}

	runtime, err := New(
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	creds, err := runtime.Resolve(
		context.Background(),
		"kept",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
	)
	if err != nil {
		t.Fatalf("Resolve() returned error: %v", err)
	}
	if creds.Username != "kept-user" || creds.Password != "kept-pass" {
		t.Fatalf("unexpected credentials %#v", creds)
	}
	if len(store.values) == 0 {
		t.Fatal("expected session store values to be persisted")
	}

	// Credential values are no longer injected into process environment;
	// the second runtime instance loads them from the shared session store.
	secondPrompter := &stubPrompter{}
	secondRuntime, err := New(
		WithPrompter(secondPrompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New(second) returned error: %v", err)
	}

	secondCreds, err := secondRuntime.Resolve(
		context.Background(),
		"kept",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(second) returned error: %v", err)
	}
	if secondCreds != creds {
		t.Fatalf("expected persisted credentials %#v, got %#v", creds, secondCreds)
	}
	if secondPrompter.promptCalls != 0 {
		t.Fatalf("expected persisted credentials to avoid prompting, got %d prompt calls", secondPrompter.promptCalls)
	}
}

func TestDefaultSessionFilePathUsesRuntimeDirOnly(t *testing.T) {
	isolatePromptAuthEnv(t)

	runtimeDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("HOME", homeDir)
	t.Setenv(promptAuthSessionIDEnv, "shell-session")

	path, err := defaultSessionFilePath()
	if err != nil {
		t.Fatalf("defaultSessionFilePath() returned error: %v", err)
	}

	want := filepath.Join(runtimeDir, "declarest", "prompt-auth", sessionFileName("shell-session"))
	if path != want {
		t.Fatalf("defaultSessionFilePath() = %q, want %q", path, want)
	}
	if strings.HasPrefix(path, homeDir) {
		t.Fatalf("expected runtime session path outside home dir, got %q", path)
	}
}

func TestNewDefaultSessionStoreRequiresRuntimeDirAndHookSessionID(t *testing.T) {
	t.Run("missing runtime dir", func(t *testing.T) {
		isolatePromptAuthEnv(t)
		t.Setenv(promptAuthSessionIDEnv, "shell-session")
		t.Setenv("XDG_RUNTIME_DIR", "")

		store, persistent, err := newDefaultSessionStore()
		if err != nil {
			t.Fatalf("newDefaultSessionStore() returned error: %v", err)
		}
		if store != nil || persistent {
			t.Fatalf("expected no persistent session store, got store=%T persistent=%t", store, persistent)
		}
	})

	t.Run("missing hook session id", func(t *testing.T) {
		isolatePromptAuthEnv(t)
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
		t.Setenv(promptAuthSessionIDEnv, "")

		store, persistent, err := newDefaultSessionStore()
		if err != nil {
			t.Fatalf("newDefaultSessionStore() returned error: %v", err)
		}
		if store != nil || persistent {
			t.Fatalf("expected no persistent session store, got store=%T persistent=%t", store, persistent)
		}
	})
}

func TestRuntimeResolveKeepsPromptedValuesAcrossRuntimeInstancesWithDefaultStore(t *testing.T) {
	isolatePromptAuthEnv(t)

	runtimeDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("HOME", homeDir)
	t.Setenv(promptAuthSessionIDEnv, "shell-session")

	firstPrompter := &stubPrompter{
		values: map[string]string{
			"shared.username": "shared-user",
			"shared.password": "shared-pass",
		},
	}
	firstRuntime, err := New(WithPrompter(firstPrompter))
	if err != nil {
		t.Fatalf("New(first) returned error: %v", err)
	}

	firstCreds, err := firstRuntime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(first) returned error: %v", err)
	}

	runtimePath, err := defaultSessionFilePath()
	if err != nil {
		t.Fatalf("defaultSessionFilePath() returned error: %v", err)
	}
	if _, err := os.Stat(runtimePath); err != nil {
		t.Fatalf("expected runtime session cache file, got %v", err)
	}

	legacyPath, err := legacySessionFilePathForSessionID(detectSessionID())
	if err != nil {
		t.Fatalf("legacySessionFilePathForSessionID() returned error: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected no home-dir session cache file, got err=%v", err)
	}

	// Credential values are no longer injected into process environment;
	// the second runtime instance loads them from the default session store.
	secondPrompter := &stubPrompter{}
	secondRuntime, err := New(WithPrompter(secondPrompter))
	if err != nil {
		t.Fatalf("New(second) returned error: %v", err)
	}

	secondCreds, err := secondRuntime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(second) returned error: %v", err)
	}

	if secondCreds != firstCreds {
		t.Fatalf("expected persisted credentials %#v, got %#v", firstCreds, secondCreds)
	}
	if secondPrompter.promptCalls != 0 {
		t.Fatalf("expected cached credentials to avoid prompting, got %d prompt calls", secondPrompter.promptCalls)
	}
}

func TestRuntimeResolveDoesNotPersistAcrossRuntimeInstancesWithoutRuntimeDir(t *testing.T) {
	isolatePromptAuthEnv(t)

	t.Setenv(promptAuthSessionIDEnv, "shell-session")
	t.Setenv("XDG_RUNTIME_DIR", "")

	firstPrompter := &stubPrompter{
		values: map[string]string{
			"shared.username": "first-user",
			"shared.password": "first-pass",
		},
	}
	firstRuntime, err := New(WithPrompter(firstPrompter))
	if err != nil {
		t.Fatalf("New(first) returned error: %v", err)
	}

	_, err = firstRuntime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(first) returned error: %v", err)
	}

	// Without XDG_RUNTIME_DIR there is no session store; credentials are
	// only held in-memory per runtime instance.
	secondPrompter := &stubPrompter{
		values: map[string]string{
			"shared.username": "second-user",
			"shared.password": "second-pass",
		},
	}
	secondRuntime, err := New(WithPrompter(secondPrompter))
	if err != nil {
		t.Fatalf("New(second) returned error: %v", err)
	}

	secondCreds, err := secondRuntime.Resolve(
		context.Background(),
		"shared",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(second) returned error: %v", err)
	}

	if secondCreds.Username != "second-user" || secondCreds.Password != "second-pass" {
		t.Fatalf("expected second runtime to prompt again without runtime session storage, got %#v", secondCreds)
	}
	if secondPrompter.promptCalls != 2 {
		t.Fatalf("expected second runtime to prompt again, got %d prompt calls", secondPrompter.promptCalls)
	}
}

func TestClearSessionCredentialsRemovesRuntimeAndLegacySessionFiles(t *testing.T) {
	isolatePromptAuthEnv(t)

	runtimeDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("HOME", homeDir)
	t.Setenv(promptAuthSessionIDEnv, "shell-session")

	runtimePath, err := defaultSessionFilePath()
	if err != nil {
		t.Fatalf("defaultSessionFilePath() returned error: %v", err)
	}
	legacyPath, err := legacySessionFilePathForSessionID(detectSessionID())
	if err != nil {
		t.Fatalf("legacySessionFilePathForSessionID() returned error: %v", err)
	}

	writeSessionValuesFile(t, runtimePath, map[string]string{"A": "runtime"})
	writeSessionValuesFile(t, legacyPath, map[string]string{"B": "legacy"})

	removed, err := ClearSessionCredentials()
	if err != nil {
		t.Fatalf("ClearSessionCredentials() returned error: %v", err)
	}
	if removed != 2 {
		t.Fatalf("ClearSessionCredentials() removed %d files, want 2", removed)
	}
	if _, err := os.Stat(runtimePath); !os.IsNotExist(err) {
		t.Fatalf("expected runtime session cache file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy session cache file to be removed, got err=%v", err)
	}
}

func TestClearSessionCredentialsRemovesLegacySessionFileWithoutHook(t *testing.T) {
	isolatePromptAuthEnv(t)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("TERM_SESSION_ID", "legacy-session")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv(promptAuthSessionIDEnv, "")

	legacyPath, err := legacySessionFilePathForSessionID(detectSessionID())
	if err != nil {
		t.Fatalf("legacySessionFilePathForSessionID() returned error: %v", err)
	}
	writeSessionValuesFile(t, legacyPath, map[string]string{"A": "legacy"})

	removed, err := ClearSessionCredentials()
	if err != nil {
		t.Fatalf("ClearSessionCredentials() returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("ClearSessionCredentials() removed %d files, want 1", removed)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy session cache file to be removed, got err=%v", err)
	}
}

func TestClearSessionCredentialsReturnsZeroWithoutSessionFiles(t *testing.T) {
	isolatePromptAuthEnv(t)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv(promptAuthSessionIDEnv, "shell-session")

	removed, err := ClearSessionCredentials()
	if err != nil {
		t.Fatalf("ClearSessionCredentials() returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("ClearSessionCredentials() removed %d files, want 0", removed)
	}
}

func TestRuntimeResolveRejectsCollidingCredentialNames(t *testing.T) {
	isolatePromptAuthEnv(t)

	store := &memorySessionStore{}
	prompter := &stubPrompter{
		values: map[string]string{
			"my-cred.username": "user-a",
			"my-cred.password": "pass-a",
			"my.cred.username": "user-b",
			"my.cred.password": "pass-b",
		},
	}

	runtime, err := New(
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	_, err = runtime.Resolve(
		context.Background(),
		"my-cred",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err != nil {
		t.Fatalf("Resolve(my-cred) returned error: %v", err)
	}

	_, err = runtime.Resolve(
		context.Background(),
		"my.cred",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true}},
	)
	if err == nil {
		t.Fatal("expected collision validation error for my.cred")
	}
	if !strings.Contains(err.Error(), "same session key") {
		t.Fatalf("expected collision error message, got: %v", err)
	}
}

func TestRuntimeResolveDoesNotLeakCredentialsToProcessEnvironment(t *testing.T) {
	isolatePromptAuthEnv(t)

	store := &memorySessionStore{}
	prompter := &stubPrompter{
		values: map[string]string{
			"secret.username": "env-check-user",
			"secret.password": "env-check-pass",
		},
	}

	runtime, err := New(
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	_, err = runtime.Resolve(
		context.Background(),
		"secret",
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
		config.CredentialValue{Prompt: &config.CredentialPrompt{Prompt: true, PersistInSession: true}},
	)
	if err != nil {
		t.Fatalf("Resolve() returned error: %v", err)
	}

	usernameKey, passwordKey := credentialEnvKeys("secret")
	if _, exists := os.LookupEnv(usernameKey); exists {
		t.Fatalf("credential username leaked to process environment via %s", usernameKey)
	}
	if _, exists := os.LookupEnv(passwordKey); exists {
		t.Fatalf("credential password leaked to process environment via %s", passwordKey)
	}
}

type stubPrompter struct {
	values      map[string]string
	promptCalls int
}

func (s *stubPrompter) PromptValue(
	_ context.Context,
	credentialName string,
	field string,
	_ bool,
	_ bool,
) (string, error) {
	s.promptCalls++
	if len(s.values) == 0 {
		return "", nil
	}
	key := credentialName + "." + field
	value, ok := s.values[key]
	if !ok {
		return "", nil
	}
	delete(s.values, key)
	return value, nil
}

type memorySessionStore struct {
	values map[string]string
}

func (s *memorySessionStore) Load() (map[string]string, error) {
	return maps.Clone(s.values), nil
}

func (s *memorySessionStore) Save(values map[string]string) error {
	s.values = maps.Clone(values)
	return nil
}

func writeSessionValuesFile(t *testing.T, path string, values map[string]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("json.Marshal() returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}

func isolatePromptAuthEnv(t *testing.T) {
	t.Helper()

	saved := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(strings.TrimSpace(key), envPrefix) {
			continue
		}
		saved[key] = value
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%q) returned error: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for key := range saved {
			_ = os.Unsetenv(key)
		}
		for key, value := range saved {
			_ = os.Setenv(key, value)
		}
	})
}
