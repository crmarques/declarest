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

package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

const (
	encryptedStoreVersion = 1
	keyLengthBytes        = 32
	nonceLengthBytes      = 12
	saltLengthBytes       = 16

	defaultKDFTime    = 3
	defaultKDFMemory  = 64 * 1024
	defaultKDFThreads = 4
)

var _ secretdomain.SecretProvider = (*Store)(nil)

type Store struct {
	path       string
	key        []byte
	passphrase []byte
	kdf        kdfSettings

	mu             sync.Mutex
	initialized    bool
	cachedSnapshot *secretSnapshot
}

func New(cfg config.FileSecretStore) (*Store, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, faults.Invalid("secret-store.file.path is required", nil)
	}

	setCount := countSet(
		strings.TrimSpace(cfg.Key) != "",
		strings.TrimSpace(cfg.KeyFile) != "",
		strings.TrimSpace(cfg.Passphrase) != "",
		strings.TrimSpace(cfg.PassphraseFile) != "",
	)
	if setCount != 1 {
		return nil, faults.Invalid("secret-store.file must define exactly one of key, key-file, passphrase, passphrase-file", nil)
	}

	kdf, err := resolveKDFSettings(cfg.KDF)
	if err != nil {
		return nil, err
	}

	service := &Store{
		path: filepath.Clean(path),
		kdf:  kdf,
	}

	switch {
	case strings.TrimSpace(cfg.Key) != "":
		key, err := parseEncryptionKey(cfg.Key)
		if err != nil {
			return nil, err
		}
		service.key = key
	case strings.TrimSpace(cfg.KeyFile) != "":
		keyFileData, err := os.ReadFile(strings.TrimSpace(cfg.KeyFile))
		if err != nil {
			return nil, faults.Invalid("secret-store.file.key-file could not be read", err)
		}
		key, err := parseEncryptionKey(string(keyFileData))
		if err != nil {
			return nil, err
		}
		service.key = key
	case strings.TrimSpace(cfg.Passphrase) != "":
		service.passphrase = []byte(strings.TrimSpace(cfg.Passphrase))
	case strings.TrimSpace(cfg.PassphraseFile) != "":
		passphraseData, err := os.ReadFile(strings.TrimSpace(cfg.PassphraseFile))
		if err != nil {
			return nil, faults.Invalid("secret-store.file.passphrase-file could not be read", err)
		}
		passphrase := strings.TrimSpace(string(passphraseData))
		if passphrase == "" {
			return nil, faults.Invalid("secret-store.file.passphrase-file must not be empty", nil)
		}
		service.passphrase = []byte(passphrase)
	default:
		return nil, faults.Invalid("secret-store.file key material is invalid", nil)
	}

	return service, nil
}

func (s *Store) Init(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked()
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

func (s *Store) initLocked() error {
	if s == nil {
		return faults.Invalid("file secret service must not be nil", nil)
	}
	if strings.TrimSpace(s.path) == "" {
		return faults.Invalid("secret store path must not be empty", nil)
	}

	if len(s.key) == 0 && len(s.passphrase) == 0 {
		return faults.Invalid("secret store key material is missing", nil)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return faults.Internal("failed to prepare secret store directory", err)
	}

	if _, err := os.Stat(s.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if writeErr := s.writeSnapshotLocked(secretSnapshot{Secrets: map[string]string{}}); writeErr != nil {
				return writeErr
			}
			s.initialized = true
			return nil
		}
		return faults.Internal("failed to inspect secret store file", err)
	}

	if !s.initialized {
		if _, err := s.readSnapshotLocked(); err != nil {
			return err
		}
	}

	s.initialized = true
	return nil
}
