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

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

const envPrefix = "DECLAREST_CONTEXT_CREDENTIAL_"

type Credentials struct {
	Username string
	Password string
}

type Prompter interface {
	PromptValue(
		ctx context.Context,
		credentialName string,
		field string,
		persistInSession bool,
		persistentSession bool,
	) (string, error)
}

type SessionStore interface {
	Load() (map[string]string, error)
	Save(values map[string]string) error
}

type Option func(*Runtime)

type Runtime struct {
	mu sync.Mutex

	prompter Prompter
	store    SessionStore

	persistentSession bool
	sessionLoaded     bool
	sessionValues     map[string]string
	warnedNoSession   bool

	resolved     map[string]Credentials
	envKeyOwners map[string]string // envKey → credential name (collision detection)
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

func New(opts ...Option) (*Runtime, error) {
	runtime := &Runtime{
		prompter: terminalPrompter{},
		resolved: map[string]Credentials{},
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
			return nil, faults.Internal("failed to initialize prompt auth session store", err)
		}
		runtime.store = store
		runtime.persistentSession = persistentSession
	}

	if err := runtime.ensureSessionLoaded(); err != nil {
		return nil, err
	}

	return runtime, nil
}

func (r *Runtime) Resolve(
	ctx context.Context,
	credentialName string,
	username config.CredentialValue,
	password config.CredentialValue,
) (Credentials, error) {
	if r == nil {
		return Credentials{}, faults.Invalid("credential runtime is not configured", nil)
	}
	if err := r.ensureSessionLoaded(); err != nil {
		return Credentials{}, err
	}

	credentialName = strings.TrimSpace(credentialName)
	if credentialName == "" {
		return Credentials{}, faults.Invalid("credential name is required", nil)
	}

	r.mu.Lock()
	if creds, ok := r.resolved[credentialName]; ok {
		r.mu.Unlock()
		return creds, nil
	}
	if err := r.registerEnvKeyOwner(credentialName); err != nil {
		r.mu.Unlock()
		return Credentials{}, err
	}
	r.mu.Unlock()

	resolvedUsername, err := r.resolveField(ctx, credentialName, "username", username)
	if err != nil {
		return Credentials{}, err
	}
	resolvedPassword, err := r.resolveField(ctx, credentialName, "password", password)
	if err != nil {
		return Credentials{}, err
	}
	creds := Credentials{
		Username: strings.TrimSpace(resolvedUsername),
		Password: strings.TrimSpace(resolvedPassword),
	}
	if creds.Username == "" || creds.Password == "" {
		return Credentials{}, faults.Invalid(
			fmt.Sprintf("credential %q requires username and password", credentialName),
			nil,
		)
	}

	r.mu.Lock()
	r.resolved[credentialName] = creds
	r.mu.Unlock()

	return creds, nil
}

func ResolveCredentials(
	runtime *Runtime,
	ctx context.Context,
	credentialName string,
	username config.CredentialValue,
	password config.CredentialValue,
) (Credentials, error) {
	if !username.IsPrompt() && !password.IsPrompt() {
		return Credentials{
			Username: username.Literal(),
			Password: password.Literal(),
		}, nil
	}
	if runtime == nil {
		return Credentials{}, faults.Invalid(
			fmt.Sprintf("credential %q requires prompt runtime support", strings.TrimSpace(credentialName)),
			nil,
		)
	}
	return runtime.Resolve(ctx, credentialName, username, password)
}

func ClearSessionCredentials() (int, error) {
	removed := 0

	runtimePath, err := defaultSessionFilePath()
	if err != nil {
		return 0, faults.Internal("failed to resolve prompt auth runtime session path", err)
	}
	if deleted, err := removeSessionCacheFile(runtimePath); err != nil {
		return 0, faults.Internal("failed to clear prompt auth runtime session cache", err)
	} else if deleted {
		removed++
	}

	legacySessionID := detectSessionID()
	legacyPath, err := legacySessionFilePathForSessionID(legacySessionID)
	if err != nil {
		return 0, faults.Internal("failed to resolve prompt auth legacy session path", err)
	}
	if legacyPath != "" && legacyPath != runtimePath {
		if deleted, err := removeSessionCacheFile(legacyPath); err != nil {
			return 0, faults.Internal("failed to clear prompt auth legacy session cache", err)
		} else if deleted {
			removed++
		}
	}

	return removed, nil
}

func (r *Runtime) resolveField(
	ctx context.Context,
	credentialName string,
	field string,
	value config.CredentialValue,
) (string, error) {
	if !value.IsPrompt() {
		return value.Literal(), nil
	}

	envKey := credentialEnvKey(credentialName, field)
	if resolved, ok := r.sessionValueFromMemory(envKey); ok {
		return resolved, nil
	}
	if resolved, ok := sessionValue(envKey); ok {
		return resolved, nil
	}

	warnOnPersistRequest := value.PersistInSession()
	if warnOnPersistRequest && !r.persistentSession {
		r.mu.Lock()
		if r.warnedNoSession {
			warnOnPersistRequest = false
		} else {
			r.warnedNoSession = true
		}
		r.mu.Unlock()
	}

	prompted, err := r.prompter.PromptValue(ctx, credentialName, field, warnOnPersistRequest, r.persistentSession)
	if err != nil {
		return "", err
	}
	prompted = strings.TrimSpace(prompted)
	if prompted == "" {
		return "", faults.Invalid(
			fmt.Sprintf("credential %q %s is required", credentialName, field),
			nil,
		)
	}
	if value.PersistInSession() {
		if err := r.persistField(envKey, prompted); err != nil {
			return "", err
		}
	}
	return prompted, nil
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
			return faults.Internal("failed to load prompt auth session credentials", err)
		}
		values = loaded
	}

	r.mu.Lock()
	r.sessionLoaded = true
	r.sessionValues = maps.Clone(values)
	r.mu.Unlock()
	return nil
}

func (r *Runtime) persistField(key string, value string) error {
	if err := r.ensureSessionLoaded(); err != nil {
		return err
	}

	r.mu.Lock()
	if r.sessionValues == nil {
		r.sessionValues = map[string]string{}
	}
	r.sessionValues[key] = value
	values := maps.Clone(r.sessionValues)
	store := r.store
	r.mu.Unlock()

	if store == nil {
		return nil
	}
	if err := store.Save(values); err != nil {
		return faults.Internal("failed to persist prompt auth session value", err)
	}
	return nil
}

// sessionValueFromMemory checks the in-memory session cache for a credential
// value. This avoids leaking credentials into the process environment.
func (r *Runtime) sessionValueFromMemory(key string) (string, bool) {
	r.mu.Lock()
	value, ok := r.sessionValues[key]
	r.mu.Unlock()
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

// registerEnvKeyOwner records the credential name that owns a given env key
// and returns an error if a different credential name already maps to the
// same key, which would cause silent cross-contamination.
// Must be called with r.mu held.
func (r *Runtime) registerEnvKeyOwner(credentialName string) error {
	if r.envKeyOwners == nil {
		r.envKeyOwners = map[string]string{}
	}
	usernameKey, passwordKey := credentialEnvKeys(credentialName)
	for _, key := range []string{usernameKey, passwordKey} {
		if owner, exists := r.envKeyOwners[key]; exists && owner != credentialName {
			return faults.Invalid(
				fmt.Sprintf(
					"credential %q and %q produce the same session key; use distinct credential names to avoid cross-contamination",
					credentialName,
					owner,
				),
				nil,
			)
		}
	}
	r.envKeyOwners[usernameKey] = credentialName
	r.envKeyOwners[passwordKey] = credentialName
	return nil
}

func credentialEnvKeys(name string) (string, string) {
	return credentialEnvKey(name, "username"), credentialEnvKey(name, "password")
}

func credentialEnvKey(name string, field string) string {
	return envPrefix + sanitizeEnvSegment(name) + "_" + strings.ToUpper(strings.TrimSpace(field))
}

func sanitizeEnvSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r - ('a' - 'A'))
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func sessionValue(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func removeSessionCacheFile(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
