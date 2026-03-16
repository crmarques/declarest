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

	"github.com/crmarques/declarest/config"
)

func TestRuntimeResolvePromptsOnceAndReusesSameCredential(t *testing.T) {
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

	usernameKey, passwordKey := credentialEnvKeys("kept")
	if err := os.Unsetenv(usernameKey); err != nil {
		t.Fatalf("Unsetenv(username) returned error: %v", err)
	}
	if err := os.Unsetenv(passwordKey); err != nil {
		t.Fatalf("Unsetenv(password) returned error: %v", err)
	}

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
