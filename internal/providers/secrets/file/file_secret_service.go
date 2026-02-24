package file

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"

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

	defaultKDFTime    = 1
	defaultKDFMemory  = 64 * 1024
	defaultKDFThreads = 4
)

var _ secretdomain.SecretProvider = (*FileSecretService)(nil)

type FileSecretService struct {
	path       string
	key        []byte
	passphrase []byte
	kdf        kdfSettings

	mu          sync.Mutex
	initialized bool
}

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
}

type secretSnapshot struct {
	Secrets map[string]string `json:"secrets"`
}

func NewFileSecretService(cfg config.FileSecretStore) (*FileSecretService, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, validationError("secret-store.file.path is required", nil)
	}

	setCount := countSet(
		strings.TrimSpace(cfg.Key) != "",
		strings.TrimSpace(cfg.KeyFile) != "",
		strings.TrimSpace(cfg.Passphrase) != "",
		strings.TrimSpace(cfg.PassphraseFile) != "",
	)
	if setCount != 1 {
		return nil, validationError("secret-store.file must define exactly one of key, key-file, passphrase, passphrase-file", nil)
	}

	kdf, err := resolveKDFSettings(cfg.KDF)
	if err != nil {
		return nil, err
	}

	service := &FileSecretService{
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
			return nil, validationError("secret-store.file.key-file could not be read", err)
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
			return nil, validationError("secret-store.file.passphrase-file could not be read", err)
		}
		passphrase := strings.TrimSpace(string(passphraseData))
		if passphrase == "" {
			return nil, validationError("secret-store.file.passphrase-file must not be empty", nil)
		}
		service.passphrase = []byte(passphrase)
	default:
		return nil, validationError("secret-store.file key material is invalid", nil)
	}

	return service, nil
}

func (s *FileSecretService) Init(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked()
}

func (s *FileSecretService) Store(_ context.Context, key string, value string) error {
	normalizedKey, err := normalizeSecretKey(key)
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

func (s *FileSecretService) Get(_ context.Context, key string) (string, error) {
	normalizedKey, err := normalizeSecretKey(key)
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
		return "", notFoundError("secret key not found")
	}
	return value, nil
}

func (s *FileSecretService) Delete(_ context.Context, key string) error {
	normalizedKey, err := normalizeSecretKey(key)
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

func (s *FileSecretService) List(context.Context) ([]string, error) {
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

func (s *FileSecretService) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return s.Store(ctx, key, secretValue)
	})
}

func (s *FileSecretService) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return s.Get(ctx, key)
	})
}

func (s *FileSecretService) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (s *FileSecretService) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

func (s *FileSecretService) initLocked() error {
	if s == nil {
		return validationError("file secret service must not be nil", nil)
	}
	if strings.TrimSpace(s.path) == "" {
		return validationError("secret store path must not be empty", nil)
	}

	if len(s.key) == 0 && len(s.passphrase) == 0 {
		return validationError("secret store key material is missing", nil)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return internalError("failed to prepare secret store directory", err)
	}

	if _, err := os.Stat(s.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if writeErr := s.writeSnapshotLocked(secretSnapshot{Secrets: map[string]string{}}); writeErr != nil {
				return writeErr
			}
			s.initialized = true
			return nil
		}
		return internalError("failed to inspect secret store file", err)
	}

	if !s.initialized {
		if _, err := s.readSnapshotLocked(); err != nil {
			return err
		}
	}

	s.initialized = true
	return nil
}

func (s *FileSecretService) readSnapshotLocked() (secretSnapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return secretSnapshot{}, internalError("failed to read encrypted secret store", err)
	}

	var envelope encryptedStore
	if err := json.Unmarshal(data, &envelope); err != nil {
		return secretSnapshot{}, internalError("failed to decode encrypted secret store", err)
	}

	if envelope.Version != encryptedStoreVersion {
		return secretSnapshot{}, validationError("secret store format version is unsupported", nil)
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return secretSnapshot{}, validationError("secret store nonce is invalid", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return secretSnapshot{}, validationError("secret store ciphertext is invalid", err)
	}

	salt := []byte(nil)
	if envelope.Salt != "" {
		salt, err = base64.StdEncoding.DecodeString(envelope.Salt)
		if err != nil {
			return secretSnapshot{}, validationError("secret store salt is invalid", err)
		}
	}

	key, err := s.deriveKey(salt)
	if err != nil {
		return secretSnapshot{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return secretSnapshot{}, internalError("failed to initialize secret cipher", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return secretSnapshot{}, internalError("failed to initialize secret cipher mode", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return secretSnapshot{}, authError("failed to decrypt secret store with provided key material", err)
	}

	var snapshot secretSnapshot
	if err := json.Unmarshal(plaintext, &snapshot); err != nil {
		return secretSnapshot{}, internalError("failed to decode decrypted secret store", err)
	}
	if snapshot.Secrets == nil {
		snapshot.Secrets = make(map[string]string)
	}

	return snapshot, nil
}

func (s *FileSecretService) writeSnapshotLocked(snapshot secretSnapshot) error {
	if snapshot.Secrets == nil {
		snapshot.Secrets = make(map[string]string)
	}

	plaintext, err := json.Marshal(snapshot)
	if err != nil {
		return internalError("failed to encode secret snapshot", err)
	}

	nonce, err := randomBytes(nonceLengthBytes)
	if err != nil {
		return internalError("failed to generate secret nonce", err)
	}

	salt := []byte(nil)
	if len(s.passphrase) > 0 {
		salt, err = randomBytes(saltLengthBytes)
		if err != nil {
			return internalError("failed to generate secret salt", err)
		}
	}

	key, err := s.deriveKey(salt)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return internalError("failed to initialize secret cipher", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return internalError("failed to initialize secret cipher mode", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	envelope := encryptedStore{
		Version:    encryptedStoreVersion,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	if len(salt) > 0 {
		envelope.Salt = base64.StdEncoding.EncodeToString(salt)
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return internalError("failed to encode encrypted secret store", err)
	}

	return writeAtomicFile(s.path, encoded, 0o600)
}

func (s *FileSecretService) deriveKey(salt []byte) ([]byte, error) {
	if len(s.key) > 0 {
		return s.key, nil
	}

	if len(s.passphrase) == 0 {
		return nil, validationError("secret store passphrase is missing", nil)
	}

	if len(salt) == 0 {
		return nil, validationError("secret store salt is missing", nil)
	}

	key := argon2.IDKey(s.passphrase, salt, s.kdf.Time, s.kdf.Memory, s.kdf.Threads, keyLengthBytes)
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
		return kdfSettings{}, validationError("secret-store.file.kdf values must be non-negative", nil)
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
		return kdfSettings{}, validationError("secret-store.file.kdf values must be greater than zero", nil)
	}

	return settings, nil
}

func parseEncryptionKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, validationError("secret-store.file.key must not be empty", nil)
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

	return nil, validationError("secret-store.file.key must be 32-byte raw, base64, or hex", nil)
}

func normalizeSecretKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "", validationError("secret key must not be empty", nil)
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", validationError("secret key contains invalid path segment", nil)
		}
	}

	return strings.Join(parts, "/"), nil
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

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return internalError("failed to create secret store directory", err)
	}

	tempFile, err := os.CreateTemp(dir, ".declarest-secret-*")
	if err != nil {
		return internalError("failed to create temporary secret file", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to write temporary secret file", err)
	}

	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return internalError("failed to set secret file permissions", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to close temporary secret file", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return internalError("failed to replace secret store file", err)
	}

	return nil
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func authError(message string, cause error) error {
	return faults.NewTypedError(faults.AuthError, message, cause)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
