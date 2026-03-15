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
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/crmarques/declarest/faults"
)

const envPrefix = "DECLAREST_PROMPT_AUTH_"

type Credentials struct {
	Username string
	Password string
}

type Prompter interface {
	PromptCredentials(ctx context.Context, target Target, keepForSession bool, persistentSession bool) (Credentials, error)
	ConfirmReuse(ctx context.Context, source Target, targets []Target) (bool, error)
}

type SessionStore interface {
	Load() (map[string]string, error)
	Save(values map[string]string) error
}

type Option func(*Runtime)

type Runtime struct {
	mu sync.Mutex

	targets map[string]Target
	order   []string

	prompter Prompter
	store    SessionStore

	persistentSession bool
	sessionLoaded     bool
	sessionValues     map[string]string

	resolved        map[string]Credentials
	shared          Credentials
	sharedSet       bool
	sharedSourceKey string
	reuseAll        bool
	reuseAsked      bool
}

func WithPrompter(prompter Prompter) Option {
	return func(runtime *Runtime) {
		if runtime == nil {
			return
		}
		runtime.prompter = prompter
	}
}

func WithSessionStore(store SessionStore) Option {
	return func(runtime *Runtime) {
		if runtime == nil {
			return
		}
		runtime.store = store
		runtime.persistentSession = store != nil
	}
}

func New(targets []Target, opts ...Option) (*Runtime, error) {
	if len(targets) == 0 {
		return nil, nil
	}

	runtime := &Runtime{
		targets:  make(map[string]Target, len(targets)),
		order:    make([]string, 0, len(targets)),
		prompter: terminalPrompter{},
		resolved: map[string]Credentials{},
	}
	for _, item := range targets {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		key := strings.TrimSpace(item.Key)
		runtime.targets[key] = item
		runtime.order = append(runtime.order, key)
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(runtime)
	}

	if runtime.store == nil {
		store, persistentSession, err := newDefaultSessionStore()
		if err != nil {
			return nil, internalError("failed to initialize prompt auth session store", err)
		}
		runtime.store = store
		runtime.persistentSession = persistentSession
	}

	if err := runtime.ensureSessionLoaded(); err != nil {
		return nil, err
	}

	return runtime, nil
}

func (r *Runtime) Resolve(ctx context.Context, key string, keepForSession bool) (Credentials, error) {
	if r == nil {
		return Credentials{}, faults.NewValidationError("prompt auth runtime is not configured", nil)
	}
	if err := r.ensureSessionLoaded(); err != nil {
		return Credentials{}, err
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return Credentials{}, faults.NewValidationError("prompt auth target is required", nil)
	}

	r.mu.Lock()
	target := r.targetLocked(key)
	if creds, ok := r.resolved[key]; ok {
		r.mu.Unlock()
		return creds, nil
	}
	if creds, ok := credentialsFromEnvironment(key); ok {
		r.resolved[key] = creds
		if !r.sharedSet {
			r.shared = creds
			r.sharedSet = true
			r.sharedSourceKey = key
		}
		r.mu.Unlock()
		return creds, nil
	}
	if r.reuseAll && r.sharedSet {
		creds := r.shared
		r.resolved[key] = creds
		r.mu.Unlock()
		if keepForSession {
			if err := r.persist(key, creds); err != nil {
				return Credentials{}, err
			}
		}
		return creds, nil
	}

	shouldAskReuseBeforePrompt := r.sharedSet && !r.reuseAsked && r.sharedSourceKey != "" && r.sharedSourceKey != key
	source := r.targetLocked(r.sharedSourceKey)
	sharedCreds := r.shared
	if shouldAskReuseBeforePrompt {
		r.reuseAsked = true
	}
	r.mu.Unlock()

	if shouldAskReuseBeforePrompt {
		reuse, err := r.prompter.ConfirmReuse(ctx, source, []Target{target})
		if err != nil {
			return Credentials{}, err
		}
		if reuse {
			r.mu.Lock()
			r.reuseAll = true
			r.resolved[key] = sharedCreds
			r.mu.Unlock()
			if keepForSession {
				if err := r.persist(key, sharedCreds); err != nil {
					return Credentials{}, err
				}
			}
			return sharedCreds, nil
		}
	}

	creds, err := r.prompter.PromptCredentials(ctx, target, keepForSession, r.persistentSession)
	if err != nil {
		return Credentials{}, err
	}
	creds.Username = strings.TrimSpace(creds.Username)
	creds.Password = strings.TrimSpace(creds.Password)
	if creds.Username == "" || creds.Password == "" {
		return Credentials{}, faults.NewValidationError(fmt.Sprintf("prompt auth for %s requires username and password", target.Label), nil)
	}

	if keepForSession {
		if err := r.persist(key, creds); err != nil {
			return Credentials{}, err
		}
	}

	r.mu.Lock()
	r.resolved[key] = creds
	if !r.sharedSet {
		r.shared = creds
		r.sharedSet = true
		r.sharedSourceKey = key
	}
	shouldAskReuseAfterPrompt := !r.reuseAsked
	others := r.pendingTargetsLocked(key)
	if shouldAskReuseAfterPrompt {
		r.reuseAsked = true
	}
	r.mu.Unlock()

	if shouldAskReuseAfterPrompt && len(others) > 0 {
		reuse, err := r.prompter.ConfirmReuse(ctx, target, others)
		if err != nil {
			return Credentials{}, err
		}
		r.mu.Lock()
		r.reuseAll = reuse
		r.mu.Unlock()
	}

	return creds, nil
}

func (r *Runtime) targetLocked(key string) Target {
	if target, ok := r.targets[key]; ok {
		return target
	}
	return target(key)
}

func (r *Runtime) pendingTargetsLocked(exclude string) []Target {
	pending := make([]Target, 0, len(r.order))
	for _, key := range r.order {
		if key == exclude {
			continue
		}
		if _, ok := r.resolved[key]; ok {
			continue
		}
		if _, ok := credentialsFromEnvironment(key); ok {
			continue
		}
		pending = append(pending, r.targetLocked(key))
	}
	return pending
}

func (r *Runtime) ensureSessionLoaded() error {
	r.mu.Lock()
	if r.sessionLoaded {
		r.mu.Unlock()
		return nil
	}
	store := r.store
	r.mu.Unlock()

	values := map[string]string{}
	if store != nil {
		loaded, err := store.Load()
		if err != nil {
			return internalError("failed to load prompt auth session credentials", err)
		}
		values = loaded
		for key, value := range loaded {
			if _, exists := os.LookupEnv(key); exists {
				continue
			}
			if err := os.Setenv(key, value); err != nil {
				return internalError("failed to apply prompt auth session credentials", err)
			}
		}
	}

	r.mu.Lock()
	r.sessionLoaded = true
	r.sessionValues = maps.Clone(values)
	r.mu.Unlock()
	return nil
}

func (r *Runtime) persist(key string, creds Credentials) error {
	if err := r.ensureSessionLoaded(); err != nil {
		return err
	}

	usernameKey, passwordKey := credentialEnvKeys(key)
	if err := os.Setenv(usernameKey, creds.Username); err != nil {
		return internalError("failed to store prompt auth session username", err)
	}
	if err := os.Setenv(passwordKey, creds.Password); err != nil {
		return internalError("failed to store prompt auth session password", err)
	}

	r.mu.Lock()
	if r.sessionValues == nil {
		r.sessionValues = map[string]string{}
	}
	r.sessionValues[usernameKey] = creds.Username
	r.sessionValues[passwordKey] = creds.Password
	values := maps.Clone(r.sessionValues)
	store := r.store
	r.mu.Unlock()

	if store == nil {
		return nil
	}
	if err := store.Save(values); err != nil {
		return internalError("failed to persist prompt auth session credentials", err)
	}
	return nil
}

func credentialEnvKeys(key string) (string, string) {
	segment := strings.ToUpper(strings.TrimSpace(key))
	return envPrefix + segment + "_USERNAME", envPrefix + segment + "_PASSWORD"
}

func credentialsFromEnvironment(key string) (Credentials, bool) {
	usernameKey, passwordKey := credentialEnvKeys(key)
	username, usernameOK := os.LookupEnv(usernameKey)
	password, passwordOK := os.LookupEnv(passwordKey)
	if !usernameOK || !passwordOK {
		return Credentials{}, false
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return Credentials{}, false
	}
	return Credentials{
		Username: username,
		Password: password,
	}, true
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
