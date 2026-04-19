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
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

type kdfSettings struct {
	Time    uint32
	Memory  uint32
	Threads uint8
}

type encryptedStore struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt,omitempty"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	KDFTime    uint32 `json:"kdf_time,omitempty"`
	KDFMemory  uint32 `json:"kdf_memory,omitempty"`
	KDFThreads uint8  `json:"kdf_threads,omitempty"`
}

func (s *Store) deriveKey(salt []byte) ([]byte, error) {
	return s.deriveKeyWithSettings(salt, s.kdf)
}

func (s *Store) deriveKeyWithSettings(salt []byte, kdf kdfSettings) ([]byte, error) {
	if len(s.key) > 0 {
		return s.key, nil
	}

	if len(s.passphrase) == 0 {
		return nil, faults.Invalid("secret store passphrase is missing", nil)
	}

	if len(salt) == 0 {
		return nil, faults.Invalid("secret store salt is missing", nil)
	}

	key := argon2.IDKey(s.passphrase, salt, kdf.Time, kdf.Memory, kdf.Threads, keyLengthBytes)
	return key, nil
}

func resolveKDFSettings(kdf *config.KDF) (kdfSettings, error) {
	settings := kdfSettings{
		Time:    defaultKDFTime,
		Memory:  defaultKDFMemory,
		Threads: defaultKDFThreads,
	}

	if kdf == nil {
		return settings, nil
	}

	if kdf.Time < 0 || kdf.Memory < 0 || kdf.Threads < 0 {
		return kdfSettings{}, faults.Invalid("secret-store.file.kdf values must be non-negative", nil)
	}

	if kdf.Time > 0 {
		settings.Time = uint32(kdf.Time)
	}
	if kdf.Memory > 0 {
		settings.Memory = uint32(kdf.Memory)
	}
	if kdf.Threads > 0 {
		settings.Threads = uint8(kdf.Threads)
	}

	if settings.Time == 0 || settings.Memory == 0 || settings.Threads == 0 {
		return kdfSettings{}, faults.Invalid("secret-store.file.kdf values must be greater than zero", nil)
	}

	return settings, nil
}

func parseEncryptionKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, faults.Invalid("secret-store.file.key must not be empty", nil)
	}

	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == keyLengthBytes {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == keyLengthBytes {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(decoded) == keyLengthBytes {
		return decoded, nil
	}

	if len(trimmed) == keyLengthBytes {
		return []byte(trimmed), nil
	}

	return nil, faults.Invalid("secret-store.file.key must be 32-byte raw, base64, or hex", nil)
}

func countSet(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func randomBytes(length int) ([]byte, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}
