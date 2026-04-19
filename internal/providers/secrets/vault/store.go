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

package vault

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

const (
	defaultVaultTimeout = 30 * time.Second
	defaultVaultMount   = "secret"
	defaultVaultKV      = 2
)

var _ secretdomain.SecretProvider = (*Store)(nil)

type vaultAuthMode int

const (
	vaultAuthToken vaultAuthMode = iota
	vaultAuthUserPass
	vaultAuthAppRole
)

type Store struct {
	address    string
	mount      string
	pathPrefix string
	kvVersion  int
	auth       vaultAuthConfig
	client     *http.Client
	runtime    *promptauth.Runtime

	mu          sync.Mutex
	token       string
	initialized bool
}

type vaultAuthConfig struct {
	mode vaultAuthMode

	token string

	userPass *config.VaultUserPasswordAuth
	appRole  *config.VaultAppRoleAuth
}

type Option func(*Store)

func WithPromptRuntime(runtime *promptauth.Runtime) Option {
	return func(service *Store) {
		if service == nil {
			return
		}
		service.runtime = runtime
	}
}

func New(cfg config.VaultSecretStore, opts ...Option) (*Store, error) {
	address, err := normalizeVaultAddress(cfg.Address)
	if err != nil {
		return nil, err
	}

	mount, err := normalizeVaultPath(cfg.Mount, true)
	if err != nil {
		return nil, faults.Invalid("secret-store.vault.mount is invalid", err)
	}
	if mount == "" {
		mount = defaultVaultMount
	}

	pathPrefix, err := normalizeVaultPath(cfg.PathPrefix, true)
	if err != nil {
		return nil, faults.Invalid("secret-store.vault.path-prefix is invalid", err)
	}

	kvVersion := cfg.KVVersion
	if kvVersion == 0 {
		kvVersion = defaultVaultKV
	}
	if kvVersion != 1 && kvVersion != 2 {
		return nil, faults.Invalid("secret-store.vault.kv-version must be 1 or 2", nil)
	}

	if cfg.Auth == nil {
		return nil, faults.Invalid("secret-store.vault.auth is required", nil)
	}

	tlsConfig, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	transport.Proxy = nil

	service := &Store{
		address:    address,
		mount:      mount,
		pathPrefix: pathPrefix,
		kvVersion:  kvVersion,
		client: &http.Client{
			Timeout:   defaultVaultTimeout,
			Transport: transport,
		},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(service)
	}

	auth, err := buildVaultAuthConfig(*cfg.Auth)
	if err != nil {
		return nil, err
	}
	service.auth = auth

	proxyConfig, disabled, err := proxyhelper.ResolveWithRuntime("secret-store.vault.proxy", cfg.Proxy, service.runtime)
	if err != nil {
		return nil, err
	}
	if !disabled {
		transport.Proxy = proxyConfig.Resolver()
	}

	if auth.mode == vaultAuthToken {
		service.token = auth.token
	}

	return service, nil
}

func (s *Store) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked(ctx)
}

func (s *Store) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return s.Store(ctx, key, secretValue)
	})
}

func (s *Store) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return s.Get(ctx, key)
	})
}

func (s *Store) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (s *Store) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

func (s *Store) ensureInitialized(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked(ctx)
}

func (s *Store) initLocked(ctx context.Context) error {
	if s == nil {
		return faults.Invalid("vault secret service must not be nil", nil)
	}
	if s.initialized {
		return nil
	}

	switch s.auth.mode {
	case vaultAuthToken:
		if strings.TrimSpace(s.auth.token) == "" {
			return faults.Auth("vault token auth requires token", nil)
		}
		s.token = strings.TrimSpace(s.auth.token)
	case vaultAuthUserPass:
		if err := s.loginUserPass(ctx); err != nil {
			return err
		}
	case vaultAuthAppRole:
		if err := s.loginAppRole(ctx); err != nil {
			return err
		}
	default:
		return faults.Invalid("vault auth mode is invalid", nil)
	}

	if strings.TrimSpace(s.token) == "" {
		return faults.Auth("vault authentication did not return a token", nil)
	}

	s.initialized = true
	return nil
}
