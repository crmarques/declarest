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
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/crmarques/declarest/faults"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type secretSnapshot struct {
	Secrets map[string]string `json:"secrets"`
}

func cloneSnapshot(src secretSnapshot) secretSnapshot {
	clone := secretSnapshot{Secrets: make(map[string]string, len(src.Secrets))}
	for k, v := range src.Secrets {
		clone.Secrets[k] = v
	}
	return clone
}

func (s *Store) Store(_ context.Context, key string, value string) error {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.initLocked(); err != nil {
		return err
	}

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	snapshot.Secrets[normalizedKey] = value

	return s.writeSnapshotLocked(snapshot)
}

func (s *Store) Get(_ context.Context, key string) (string, error) {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.initLocked(); err != nil {
		return "", err
	}

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return "", err
	}

	value, found := snapshot.Secrets[normalizedKey]
	if !found {
		return "", faults.NotFound("secret key not found", nil)
	}
	return value, nil
}

func (s *Store) Delete(_ context.Context, key string) error {
	normalizedKey, err := secretdomain.NormalizeKey(key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.initLocked(); err != nil {
		return err
	}

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return err
	}
	delete(snapshot.Secrets, normalizedKey)

	return s.writeSnapshotLocked(snapshot)
}

func (s *Store) List(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.initLocked(); err != nil {
		return nil, err
	}

	snapshot, err := s.readSnapshotLocked()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(snapshot.Secrets))
	for key := range snapshot.Secrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys, nil
}

func (s *Store) readSnapshotLocked() (secretSnapshot, error) {
	if s.cachedSnapshot != nil {
		return cloneSnapshot(*s.cachedSnapshot), nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return secretSnapshot{}, faults.Internal("failed to read encrypted secret store", err)
	}

	var envelope encryptedStore
	if err := json.Unmarshal(data, &envelope); err != nil {
		return secretSnapshot{}, faults.Internal("failed to decode encrypted secret store", err)
	}

	if envelope.Version != encryptedStoreVersion {
		return secretSnapshot{}, faults.Invalid("secret store format version is unsupported", nil)
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return secretSnapshot{}, faults.Invalid("secret store nonce is invalid", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return secretSnapshot{}, faults.Invalid("secret store ciphertext is invalid", err)
	}

	salt := []byte(nil)
	if envelope.Salt != "" {
		salt, err = base64.StdEncoding.DecodeString(envelope.Salt)
		if err != nil {
			return secretSnapshot{}, faults.Invalid("secret store salt is invalid", err)
		}
	}

	readKDF := s.kdf
	if len(salt) > 0 {
		if envelope.KDFTime == 0 || envelope.KDFMemory == 0 || envelope.KDFThreads == 0 {
			return secretSnapshot{}, faults.Invalid("secret store KDF parameters are missing", nil)
		}
		readKDF = kdfSettings{
			Time:    envelope.KDFTime,
			Memory:  envelope.KDFMemory,
			Threads: envelope.KDFThreads,
		}
	}

	key, err := s.deriveKeyWithSettings(salt, readKDF)
	if err != nil {
		return secretSnapshot{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return secretSnapshot{}, faults.Internal("failed to initialize secret cipher", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return secretSnapshot{}, faults.Internal("failed to initialize secret cipher mode", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return secretSnapshot{}, faults.Auth("failed to decrypt secret store with provided key material", err)
	}

	var snapshot secretSnapshot
	if err := json.Unmarshal(plaintext, &snapshot); err != nil {
		return secretSnapshot{}, faults.Internal("failed to decode decrypted secret store", err)
	}
	if snapshot.Secrets == nil {
		snapshot.Secrets = make(map[string]string)
	}

	s.cachedSnapshot = &snapshot
	return cloneSnapshot(snapshot), nil
}

func (s *Store) writeSnapshotLocked(snapshot secretSnapshot) error {
	if snapshot.Secrets == nil {
		snapshot.Secrets = make(map[string]string)
	}

	plaintext, err := json.Marshal(snapshot)
	if err != nil {
		return faults.Internal("failed to encode secret snapshot", err)
	}

	nonce, err := randomBytes(nonceLengthBytes)
	if err != nil {
		return faults.Internal("failed to generate secret nonce", err)
	}

	salt := []byte(nil)
	if len(s.passphrase) > 0 {
		salt, err = randomBytes(saltLengthBytes)
		if err != nil {
			return faults.Internal("failed to generate secret salt", err)
		}
	}

	key, err := s.deriveKey(salt)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return faults.Internal("failed to initialize secret cipher", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return faults.Internal("failed to initialize secret cipher mode", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	envelope := encryptedStore{
		Version:    encryptedStoreVersion,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	if len(salt) > 0 {
		envelope.Salt = base64.StdEncoding.EncodeToString(salt)
		envelope.KDFTime = s.kdf.Time
		envelope.KDFMemory = s.kdf.Memory
		envelope.KDFThreads = s.kdf.Threads
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return faults.Internal("failed to encode encrypted secret store", err)
	}

	if err := writeAtomicFile(s.path, encoded, 0o600); err != nil {
		return err
	}
	s.cachedSnapshot = &snapshot
	return nil
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return faults.Internal("failed to create secret store directory", err)
	}

	tempFile, err := os.CreateTemp(dir, ".declarest-secret-*")
	if err != nil {
		return faults.Internal("failed to create temporary secret file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return faults.Internal("failed to write temporary secret file", err)
	}

	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return faults.Internal("failed to set secret file permissions", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return faults.Internal("failed to close temporary secret file", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return faults.Internal("failed to replace secret store file", err)
	}

	return nil
}
