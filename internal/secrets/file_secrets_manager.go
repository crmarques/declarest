package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"declarest/internal/resource"

	"golang.org/x/crypto/argon2"
)

const (
	fileSecretsVersion     = 1
	fileSecretsCipherName  = "aes-256-gcm"
	fileSecretsKDFRaw      = "raw"
	fileSecretsKDFArgon2id = "argon2id"
	fileSecretsKeyLen      = 32
	fileSecretsNonceLen    = 12
)

type FileSecretsManager struct {
	cfg         *FileSecretsManagerConfig
	path        string
	key         []byte
	kdf         fileSecretsKDF
	store       fileSecretsStore
	initialized bool
	mu          sync.RWMutex
}

type fileSecretsPayload struct {
	Version int               `json:"version"`
	KDF     fileSecretsKDF    `json:"kdf"`
	Cipher  fileSecretsCipher `json:"cipher"`
	Data    string            `json:"data"`
}

type fileSecretsKDF struct {
	Name    string `json:"name"`
	Salt    string `json:"salt,omitempty"`
	Time    uint32 `json:"time,omitempty"`
	Memory  uint32 `json:"memory,omitempty"`
	Threads uint8  `json:"threads,omitempty"`
	KeyLen  uint32 `json:"key_len,omitempty"`
}

type fileSecretsCipher struct {
	Name  string `json:"name"`
	Nonce string `json:"nonce"`
}

type fileSecretsStore struct {
	Resources map[string]map[string]string `json:"resources"`
}

type keySourceKind int

const (
	keySourceNone keySourceKind = iota
	keySourceRaw
	keySourcePassphrase
)

type keySource struct {
	kind  keySourceKind
	value string
}

func NewFileSecretsManager(cfg *FileSecretsManagerConfig) *FileSecretsManager {
	return &FileSecretsManager{cfg: cfg}
}

func (m *FileSecretsManager) Init() error {
	if m == nil {
		return errors.New("file secret store is nil")
	}
	if m.cfg == nil {
		return errors.New("secret_store.file configuration is required in the context")
	}
	trimmed := strings.TrimSpace(m.cfg.Path)
	if trimmed == "" {
		return errors.New("secret_store.file.path is required in the secret_store configuration")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("failed to resolve secrets file path: %w", err)
	}
	m.path = abs

	source, err := m.keySource()
	if err != nil {
		return err
	}

	payload, exists, err := m.readPayloadIfExists()
	if err != nil {
		return err
	}

	if exists {
		if err := m.ensureFilePermissions(); err != nil {
			return err
		}
		key, err := deriveKey(source, payload.KDF)
		if err != nil {
			return err
		}
		store, err := decryptStore(payload, key)
		if err != nil {
			return err
		}
		m.key = key
		m.kdf = payload.KDF
		m.store = store
		m.initialized = true
		return nil
	}

	kdf, key, err := buildKDF(source, m.cfg)
	if err != nil {
		return err
	}
	m.key = key
	m.kdf = kdf
	m.store = fileSecretsStore{Resources: map[string]map[string]string{}}
	m.initialized = true
	return nil
}

func (m *FileSecretsManager) GetSecret(resourcePath string, key string) (string, error) {
	if err := m.ensureInit(); err != nil {
		return "", err
	}
	path := resource.NormalizePath(resourcePath)
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", errors.New("secret key is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	resources := m.store.Resources
	if resources == nil {
		return "", fs.ErrNotExist
	}
	entries, ok := resources[path]
	if !ok {
		return "", fs.ErrNotExist
	}
	value, ok := entries[trimmedKey]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (m *FileSecretsManager) CreateSecret(resourcePath string, key string, value string) error {
	return m.setSecret(resourcePath, key, value)
}

func (m *FileSecretsManager) UpdateSecret(resourcePath string, key string, value string) error {
	return m.setSecret(resourcePath, key, value)
}

func (m *FileSecretsManager) DeleteSecret(resourcePath string, key string, _ string) error {
	if err := m.ensureInit(); err != nil {
		return err
	}
	path := resource.NormalizePath(resourcePath)
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return errors.New("secret key is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	resources := m.store.Resources
	if resources == nil {
		return fs.ErrNotExist
	}
	entries, ok := resources[path]
	if !ok {
		return fs.ErrNotExist
	}
	if _, ok := entries[trimmedKey]; !ok {
		return fs.ErrNotExist
	}
	delete(entries, trimmedKey)
	if len(entries) == 0 {
		delete(resources, path)
	}
	return m.persistLocked()
}

func (m *FileSecretsManager) ListKeys(resourcePath string) []string {
	if m == nil || !m.initialized {
		return []string{}
	}
	path := resource.NormalizePath(resourcePath)

	m.mu.RLock()
	defer m.mu.RUnlock()

	resources := m.store.Resources
	if resources == nil {
		return []string{}
	}
	entries, ok := resources[path]
	if !ok {
		return []string{}
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *FileSecretsManager) ListResources() ([]string, error) {
	if err := m.ensureInit(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	resources := m.store.Resources
	if resources == nil {
		return []string{}, nil
	}
	keys := make([]string, 0, len(resources))
	for path := range resources {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *FileSecretsManager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.key {
		m.key[i] = 0
	}
	m.key = nil
	m.store = fileSecretsStore{}
	m.initialized = false
	return nil
}

func (m *FileSecretsManager) EnsureFile() error {
	if err := m.ensureInit(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.path); err == nil {
		return m.ensureFilePermissions()
	} else if errors.Is(err, fs.ErrNotExist) {
		return m.persistLocked()
	} else {
		return err
	}
}

func (m *FileSecretsManager) ensureInit() error {
	if m == nil {
		return errors.New("file secret store is nil")
	}
	if !m.initialized {
		return ErrSecretStoreNotInitialized
	}
	return nil
}

func (m *FileSecretsManager) setSecret(resourcePath string, key string, value string) error {
	if err := m.ensureInit(); err != nil {
		return err
	}
	path := resource.NormalizePath(resourcePath)
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return errors.New("secret key is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store.Resources == nil {
		m.store.Resources = map[string]map[string]string{}
	}
	entries := m.store.Resources[path]
	if entries == nil {
		entries = map[string]string{}
		m.store.Resources[path] = entries
	}
	entries[trimmedKey] = value

	return m.persistLocked()
}

func (m *FileSecretsManager) persistLocked() error {
	if m.path == "" {
		return errors.New("secret_store.file.path must be configured to persist the secrets file")
	}
	payload, err := encryptStore(m.store, m.key, m.kdf)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".declarest-secrets-*")
	if err != nil {
		return fmt.Errorf("failed to create secrets temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), m.path); err != nil {
		return err
	}
	return os.Chmod(m.path, 0o600)
}

func (m *FileSecretsManager) readPayloadIfExists() (fileSecretsPayload, bool, error) {
	var payload fileSecretsPayload
	info, err := os.Stat(m.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return payload, false, nil
		}
		return payload, false, err
	}
	if info.IsDir() {
		return payload, false, fmt.Errorf("secrets file path %q is a directory", m.path)
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		return payload, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return payload, false, errors.New("secrets file is empty")
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, false, fmt.Errorf("failed to parse secrets file: %w", err)
	}
	return payload, true, nil
}

func (m *FileSecretsManager) keySource() (keySource, error) {
	key := strings.TrimSpace(m.cfg.Key)
	keyFile := strings.TrimSpace(m.cfg.KeyFile)
	passphrase := strings.TrimSpace(m.cfg.Passphrase)
	passphraseFile := strings.TrimSpace(m.cfg.PassphraseFile)

	if keyFile != "" && key != "" {
		return keySource{}, errors.New("only one of secret_store.file.key or secret_store.file.key_file may be set")
	}
	if passphraseFile != "" && passphrase != "" {
		return keySource{}, errors.New("only one of secret_store.file.passphrase or secret_store.file.passphrase_file may be set")
	}

	if keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return keySource{}, fmt.Errorf("failed to read secrets key file: %w", err)
		}
		key = strings.TrimSpace(string(data))
	}
	if passphraseFile != "" {
		data, err := os.ReadFile(passphraseFile)
		if err != nil {
			return keySource{}, fmt.Errorf("failed to read secrets passphrase file: %w", err)
		}
		passphrase = strings.TrimSpace(string(data))
	}

	if key != "" && passphrase != "" {
		return keySource{}, errors.New("provide either secret_store.file.key (or key_file) or secret_store.file.passphrase (or passphrase_file)")
	}
	if key != "" {
		return keySource{kind: keySourceRaw, value: key}, nil
	}
	if passphrase != "" {
		return keySource{kind: keySourcePassphrase, value: passphrase}, nil
	}
	return keySource{}, errors.New("set secret_store.file.key (or key_file) or secret_store.file.passphrase (or passphrase_file)")
}

func (m *FileSecretsManager) ensureFilePermissions() error {
	info, err := os.Stat(m.path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(m.path, 0o600); err != nil {
			return fmt.Errorf("failed to secure secrets file permissions: %w", err)
		}
	}
	return nil
}

func buildKDF(source keySource, cfg *FileSecretsManagerConfig) (fileSecretsKDF, []byte, error) {
	switch source.kind {
	case keySourceRaw:
		kdf := fileSecretsKDF{
			Name:   fileSecretsKDFRaw,
			KeyLen: fileSecretsKeyLen,
		}
		key, err := deriveKey(source, kdf)
		return kdf, key, err
	case keySourcePassphrase:
		kdf := fileSecretsKDF{
			Name:    fileSecretsKDFArgon2id,
			Time:    defaultKDFTime(cfg),
			Memory:  defaultKDFMemory(cfg),
			Threads: defaultKDFThreads(cfg),
			KeyLen:  fileSecretsKeyLen,
		}
		salt := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return fileSecretsKDF{}, nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		kdf.Salt = base64.StdEncoding.EncodeToString(salt)
		key, err := deriveKey(source, kdf)
		return kdf, key, err
	default:
		return fileSecretsKDF{}, nil, errors.New("invalid key source")
	}
}

func deriveKey(source keySource, kdf fileSecretsKDF) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(kdf.Name)) {
	case fileSecretsKDFRaw:
		if source.kind != keySourceRaw {
			return nil, errors.New("secrets file expects a raw key")
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(source.value))
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 key: %w", err)
		}
		if len(raw) != fileSecretsKeyLen {
			return nil, fmt.Errorf("key must be %d bytes after base64 decoding", fileSecretsKeyLen)
		}
		return raw, nil
	case fileSecretsKDFArgon2id:
		if source.kind != keySourcePassphrase {
			return nil, errors.New("secrets file expects a passphrase")
		}
		if kdf.Salt == "" {
			return nil, errors.New("secrets file is missing KDF salt")
		}
		salt, err := base64.StdEncoding.DecodeString(kdf.Salt)
		if err != nil {
			return nil, fmt.Errorf("failed to decode KDF salt: %w", err)
		}
		if kdf.Time == 0 || kdf.Memory == 0 || kdf.Threads == 0 {
			return nil, errors.New("invalid KDF parameters")
		}
		keyLen := kdf.KeyLen
		if keyLen == 0 {
			keyLen = fileSecretsKeyLen
		}
		key := argon2.IDKey([]byte(source.value), salt, kdf.Time, kdf.Memory, kdf.Threads, keyLen)
		if len(key) != fileSecretsKeyLen {
			return nil, fmt.Errorf("derived key length must be %d bytes", fileSecretsKeyLen)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported KDF %q", kdf.Name)
	}
}

func encryptStore(store fileSecretsStore, key []byte, kdf fileSecretsKDF) (fileSecretsPayload, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return fileSecretsPayload{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fileSecretsPayload{}, err
	}
	nonce := make([]byte, fileSecretsNonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fileSecretsPayload{}, err
	}
	plain, err := json.Marshal(store)
	if err != nil {
		return fileSecretsPayload{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)

	return fileSecretsPayload{
		Version: fileSecretsVersion,
		KDF:     kdf,
		Cipher: fileSecretsCipher{
			Name:  fileSecretsCipherName,
			Nonce: base64.StdEncoding.EncodeToString(nonce),
		},
		Data: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func decryptStore(payload fileSecretsPayload, key []byte) (fileSecretsStore, error) {
	if payload.Version != fileSecretsVersion {
		return fileSecretsStore{}, fmt.Errorf("unsupported secrets file version %d", payload.Version)
	}
	if strings.TrimSpace(payload.Cipher.Name) != fileSecretsCipherName {
		return fileSecretsStore{}, fmt.Errorf("unsupported cipher %q", payload.Cipher.Name)
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Cipher.Nonce)
	if err != nil {
		return fileSecretsStore{}, fmt.Errorf("failed to decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return fileSecretsStore{}, fmt.Errorf("failed to decode secrets data: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fileSecretsStore{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fileSecretsStore{}, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fileSecretsStore{}, fmt.Errorf("failed to decrypt secrets: %w", err)
	}
	var store fileSecretsStore
	if err := json.Unmarshal(plain, &store); err != nil {
		return fileSecretsStore{}, fmt.Errorf("failed to decode secrets payload: %w", err)
	}
	if store.Resources == nil {
		store.Resources = map[string]map[string]string{}
	}
	return store, nil
}

func defaultKDFTime(cfg *FileSecretsManagerConfig) uint32 {
	if cfg != nil && cfg.KDF != nil && cfg.KDF.Time > 0 {
		return cfg.KDF.Time
	}
	return 3
}

func defaultKDFMemory(cfg *FileSecretsManagerConfig) uint32 {
	if cfg != nil && cfg.KDF != nil && cfg.KDF.Memory > 0 {
		return cfg.KDF.Memory
	}
	return 64 * 1024
}

func defaultKDFThreads(cfg *FileSecretsManagerConfig) uint8 {
	if cfg != nil && cfg.KDF != nil && cfg.KDF.Threads > 0 {
		return cfg.KDF.Threads
	}
	threads := runtime.NumCPU()
	if threads < 1 {
		threads = 1
	}
	if threads > 4 {
		threads = 4
	}
	return uint8(threads)
}
