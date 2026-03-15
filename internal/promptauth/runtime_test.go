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
	"maps"
	"os"
	"testing"
)

func TestRuntimeResolvePromptsOnceAndReusesForOtherTargets(t *testing.T) {
	store := &memorySessionStore{}
	prompter := &stubPrompter{
		credentials: []Credentials{{Username: "shared-user", Password: "shared-pass"}},
		reuse:       []bool{true},
	}

	runtime, err := New(
		[]Target{
			{Key: TargetManagedServerHTTPAuth, Label: "managed-server auth"},
			{Key: TargetSecretStoreVaultAuth, Label: "vault auth"},
		},
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	first, err := runtime.Resolve(context.Background(), TargetManagedServerHTTPAuth, false)
	if err != nil {
		t.Fatalf("Resolve(first) returned error: %v", err)
	}
	second, err := runtime.Resolve(context.Background(), TargetSecretStoreVaultAuth, false)
	if err != nil {
		t.Fatalf("Resolve(second) returned error: %v", err)
	}

	if first != second {
		t.Fatalf("expected reused credentials, got %#v and %#v", first, second)
	}
	if prompter.promptCalls != 1 {
		t.Fatalf("expected one credential prompt, got %d", prompter.promptCalls)
	}
	if prompter.confirmCalls != 1 {
		t.Fatalf("expected one reuse confirmation, got %d", prompter.confirmCalls)
	}
}

func TestRuntimeResolveKeepsCredentialsForSession(t *testing.T) {
	store := &memorySessionStore{}
	prompter := &stubPrompter{
		credentials: []Credentials{{Username: "kept-user", Password: "kept-pass"}},
	}

	runtime, err := New(
		[]Target{{Key: TargetManagedServerHTTPAuth, Label: "managed-server auth"}},
		WithPrompter(prompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	creds, err := runtime.Resolve(context.Background(), TargetManagedServerHTTPAuth, true)
	if err != nil {
		t.Fatalf("Resolve() returned error: %v", err)
	}
	if creds.Username != "kept-user" || creds.Password != "kept-pass" {
		t.Fatalf("unexpected credentials %#v", creds)
	}
	if len(store.values) == 0 {
		t.Fatal("expected session store values to be persisted")
	}

	usernameKey, passwordKey := credentialEnvKeys(TargetManagedServerHTTPAuth)
	if err := os.Unsetenv(usernameKey); err != nil {
		t.Fatalf("Unsetenv(username) returned error: %v", err)
	}
	if err := os.Unsetenv(passwordKey); err != nil {
		t.Fatalf("Unsetenv(password) returned error: %v", err)
	}

	secondPrompter := &stubPrompter{}
	secondRuntime, err := New(
		[]Target{{Key: TargetManagedServerHTTPAuth, Label: "managed-server auth"}},
		WithPrompter(secondPrompter),
		WithSessionStore(store),
	)
	if err != nil {
		t.Fatalf("New(second) returned error: %v", err)
	}

	secondCreds, err := secondRuntime.Resolve(context.Background(), TargetManagedServerHTTPAuth, false)
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

type stubPrompter struct {
	credentials []Credentials
	reuse       []bool

	promptCalls  int
	confirmCalls int
}

func (s *stubPrompter) PromptCredentials(context.Context, Target, bool, bool) (Credentials, error) {
	s.promptCalls++
	if len(s.credentials) == 0 {
		return Credentials{}, nil
	}
	creds := s.credentials[0]
	s.credentials = s.credentials[1:]
	return creds, nil
}

func (s *stubPrompter) ConfirmReuse(context.Context, Target, []Target) (bool, error) {
	s.confirmCalls++
	if len(s.reuse) == 0 {
		return false, nil
	}
	value := s.reuse[0]
	s.reuse = s.reuse[1:]
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
