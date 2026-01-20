package secrets

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/crmarques/declarest/resource"
)

const (
	defaultVaultMount             = "secret"
	defaultVaultKVVersion         = 2
	defaultVaultAuthMountUserpass = "userpass"
	defaultVaultAuthMountAppRole  = "approle"
	vaultRootResourcePlaceholder  = "_root"
	vaultDefaultRequestTimeout    = 30 * time.Second
)

type VaultSecretsManager struct {
	cfg         *VaultSecretsManagerConfig
	client      *http.Client
	address     string
	mount       string
	pathPrefix  string
	kvVersion   int
	token       string
	initialized bool
	mu          sync.RWMutex
}

type vaultAuthResponse struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

type vaultKVv2ReadResponse struct {
	Data struct {
		Data map[string]any `json:"data"`
	} `json:"data"`
}

type vaultKVv1ReadResponse struct {
	Data map[string]any `json:"data"`
}

type vaultListResponse struct {
	Data struct {
		Keys []string `json:"keys"`
	} `json:"data"`
}

func NewVaultSecretsManager(cfg *VaultSecretsManagerConfig) *VaultSecretsManager {
	return &VaultSecretsManager{cfg: cfg}
}

func (m *VaultSecretsManager) Init() error {
	if m == nil {
		return errors.New("vault secret store is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg == nil {
		return errors.New("vault secret store config is required")
	}

	address := strings.TrimSpace(m.cfg.Address)
	if address == "" {
		return errors.New("vault address is required")
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid vault address %q", address)
	}
	address = strings.TrimSuffix(address, "/")

	mount := strings.Trim(m.cfg.Mount, "/")
	if mount == "" {
		mount = defaultVaultMount
	}

	kvVersion := m.cfg.KVVersion
	if kvVersion == 0 {
		kvVersion = defaultVaultKVVersion
	}
	if kvVersion != 1 && kvVersion != 2 {
		return fmt.Errorf("unsupported vault kv version %d", kvVersion)
	}

	pathPrefix := strings.Trim(m.cfg.PathPrefix, "/")

	client, err := buildVaultHTTPClient(m.cfg.TLS)
	if err != nil {
		return err
	}

	token, err := m.resolveAuthToken(client, address)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("vault authentication did not return a token")
	}

	m.client = client
	m.address = address
	m.mount = mount
	m.pathPrefix = pathPrefix
	m.kvVersion = kvVersion
	m.token = token
	m.initialized = true
	return nil
}

func (m *VaultSecretsManager) EnsureFile() error {
	if m == nil {
		return errors.New("vault secret store is nil")
	}
	if err := m.ensureInit(); err != nil {
		return m.Init()
	}
	return nil
}

func (m *VaultSecretsManager) GetSecret(resourcePath string, key string) (string, error) {
	if err := m.ensureInit(); err != nil {
		return "", err
	}
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", errors.New("secret key is required")
	}
	entries, err := m.readSecretData(resourcePath)
	if err != nil {
		return "", err
	}
	value, ok := entries[trimmedKey]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (m *VaultSecretsManager) SetSecret(resourcePath string, key string, value string) error {
	return m.setSecret(resourcePath, key, value)
}

func (m *VaultSecretsManager) DeleteSecret(resourcePath string, key string) error {
	if err := m.ensureInit(); err != nil {
		return err
	}
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return errors.New("secret key is required")
	}

	entries, err := m.readSecretData(resourcePath)
	if err != nil {
		return err
	}
	if _, ok := entries[trimmedKey]; !ok {
		return fs.ErrNotExist
	}
	delete(entries, trimmedKey)
	if len(entries) == 0 {
		return m.deleteSecretPath(resourcePath)
	}
	return m.writeSecretData(resourcePath, entries)
}

func (m *VaultSecretsManager) ListKeys(resourcePath string) ([]string, error) {
	if err := m.ensureInit(); err != nil {
		return nil, err
	}
	entries, err := m.readSecretData(resourcePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *VaultSecretsManager) ListResources() ([]string, error) {
	if err := m.ensureInit(); err != nil {
		return nil, err
	}

	results := []string{}
	if err := m.listResourcesRecursive("", &results); err != nil {
		return nil, err
	}
	sort.Strings(results)
	return results, nil
}

func (m *VaultSecretsManager) Close() error {
	if m == nil {
		return nil
	}
	return nil
}

func (m *VaultSecretsManager) setSecret(resourcePath string, key string, value string) error {
	if err := m.ensureInit(); err != nil {
		return err
	}
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return errors.New("secret key is required")
	}
	entries, err := m.readSecretData(resourcePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		entries = map[string]string{}
	}
	entries[trimmedKey] = value
	return m.writeSecretData(resourcePath, entries)
}

func (m *VaultSecretsManager) ensureInit() error {
	if m == nil {
		return errors.New("vault secret store is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.initialized {
		return ErrSecretStoreNotInitialized
	}
	return nil
}

func (m *VaultSecretsManager) resolveAuthToken(client *http.Client, address string) (string, error) {
	auth := m.cfg.Auth
	if auth == nil {
		return "", errors.New("vault auth configuration is required")
	}

	count := 0
	if strings.TrimSpace(auth.Token) != "" {
		count++
	}
	if auth.Password != nil {
		count++
	}
	if auth.AppRole != nil {
		count++
	}
	if count == 0 {
		return "", errors.New("vault auth configuration is required")
	}
	if count > 1 {
		return "", errors.New("vault auth configuration must define exactly one of token, password, or approle")
	}

	if strings.TrimSpace(auth.Token) != "" {
		return strings.TrimSpace(auth.Token), nil
	}
	if auth.Password != nil {
		return loginVaultUserpass(client, address, auth.Password)
	}
	return loginVaultAppRole(client, address, auth.AppRole)
}

func buildVaultHTTPClient(cfg *VaultSecretsManagerTLSConfig) (*http.Client, error) {
	transport := &http.Transport{}
	if cfg != nil {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		if strings.TrimSpace(cfg.CACertFile) != "" {
			pemData, err := os.ReadFile(cfg.CACertFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read vault CA certificate: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pemData) {
				return nil, errors.New("failed to parse vault CA certificate")
			}
			tlsConfig.RootCAs = pool
		}
		if strings.TrimSpace(cfg.ClientCertFile) != "" || strings.TrimSpace(cfg.ClientKeyFile) != "" {
			if strings.TrimSpace(cfg.ClientCertFile) == "" || strings.TrimSpace(cfg.ClientKeyFile) == "" {
				return nil, errors.New("vault client cert and key files are both required for mTLS")
			}
			cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load vault client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		transport.TLSClientConfig = tlsConfig
	}

	return &http.Client{
		Transport: transport,
		Timeout:   vaultDefaultRequestTimeout,
	}, nil
}

func loginVaultUserpass(client *http.Client, address string, cfg *VaultSecretsManagerPasswordAuthConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("vault password auth configuration is required")
	}
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	if username == "" || password == "" {
		return "", errors.New("vault username and password are required")
	}
	mount := strings.Trim(cfg.Mount, "/")
	if mount == "" {
		mount = defaultVaultAuthMountUserpass
	}
	apiPath := path.Join("auth", mount, "login", username)
	payload := map[string]string{"password": password}
	var resp vaultAuthResponse
	if err := vaultLoginRequest(client, address, apiPath, payload, &resp); err != nil {
		return "", err
	}
	token := strings.TrimSpace(resp.Auth.ClientToken)
	if token == "" {
		return "", errors.New("vault userpass auth did not return a token")
	}
	return token, nil
}

func loginVaultAppRole(client *http.Client, address string, cfg *VaultSecretsManagerAppRoleAuthConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("vault approle auth configuration is required")
	}
	roleID := strings.TrimSpace(cfg.RoleID)
	secretID := strings.TrimSpace(cfg.SecretID)
	if roleID == "" || secretID == "" {
		return "", errors.New("vault role_id and secret_id are required")
	}
	mount := strings.Trim(cfg.Mount, "/")
	if mount == "" {
		mount = defaultVaultAuthMountAppRole
	}
	apiPath := path.Join("auth", mount, "login")
	payload := map[string]string{
		"role_id":   roleID,
		"secret_id": secretID,
	}
	var resp vaultAuthResponse
	if err := vaultLoginRequest(client, address, apiPath, payload, &resp); err != nil {
		return "", err
	}
	token := strings.TrimSpace(resp.Auth.ClientToken)
	if token == "" {
		return "", errors.New("vault approle auth did not return a token")
	}
	return token, nil
}

func vaultLoginRequest(client *http.Client, address string, apiPath string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := strings.TrimSuffix(address, "/") + "/v1/" + strings.TrimPrefix(apiPath, "/")
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vault auth request failed: %s", strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func (m *VaultSecretsManager) readSecretData(resourcePath string) (map[string]string, error) {
	apiPath := m.secretReadPath(resourcePath)
	if apiPath == "" {
		return nil, errors.New("vault secret path is required")
	}

	if m.kvVersion == 2 {
		var resp vaultKVv2ReadResponse
		if err := m.requestJSON(http.MethodGet, apiPath, nil, &resp); err != nil {
			return nil, err
		}
		return stringifySecretData(resp.Data.Data)
	}

	var resp vaultKVv1ReadResponse
	if err := m.requestJSON(http.MethodGet, apiPath, nil, &resp); err != nil {
		return nil, err
	}
	return stringifySecretData(resp.Data)
}

func (m *VaultSecretsManager) writeSecretData(resourcePath string, entries map[string]string) error {
	apiPath := m.secretWritePath(resourcePath)
	if apiPath == "" {
		return errors.New("vault secret path is required")
	}
	payload := map[string]any{}
	if m.kvVersion == 2 {
		payload["data"] = entries
	} else {
		for key, value := range entries {
			payload[key] = value
		}
	}
	return m.requestJSON(http.MethodPost, apiPath, payload, nil)
}

func (m *VaultSecretsManager) deleteSecretPath(resourcePath string) error {
	apiPath := m.secretDeletePath(resourcePath)
	if apiPath == "" {
		return errors.New("vault secret path is required")
	}
	return m.requestJSON(http.MethodDelete, apiPath, nil, nil)
}

func (m *VaultSecretsManager) listResourcesRecursive(rel string, results *[]string) error {
	keys, err := m.listSecretKeys(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			next := path.Join(rel, strings.TrimSuffix(key, "/"))
			if err := m.listResourcesRecursive(next, results); err != nil {
				return err
			}
			continue
		}
		full := path.Join(rel, key)
		*results = append(*results, m.resourcePathFromRel(full))
	}
	return nil
}

func (m *VaultSecretsManager) listSecretKeys(rel string) ([]string, error) {
	storagePath := m.storagePathFromRel(rel)
	apiPath := m.secretListPath(storagePath)
	if apiPath == "" {
		return nil, errors.New("vault list path is required")
	}
	apiPath = apiPath + "?list=true"
	var resp vaultListResponse
	if err := m.requestJSON(http.MethodGet, apiPath, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data.Keys, nil
}

func (m *VaultSecretsManager) secretReadPath(resourcePath string) string {
	storagePath := m.storagePath(resourcePath)
	if storagePath == "" {
		return ""
	}
	if m.kvVersion == 2 {
		return path.Join(m.mount, "data", storagePath)
	}
	return path.Join(m.mount, storagePath)
}

func (m *VaultSecretsManager) secretWritePath(resourcePath string) string {
	return m.secretReadPath(resourcePath)
}

func (m *VaultSecretsManager) secretDeletePath(resourcePath string) string {
	storagePath := m.storagePath(resourcePath)
	if storagePath == "" {
		return ""
	}
	if m.kvVersion == 2 {
		return path.Join(m.mount, "metadata", storagePath)
	}
	return path.Join(m.mount, storagePath)
}

func (m *VaultSecretsManager) secretListPath(storagePath string) string {
	if m.kvVersion == 2 {
		if storagePath == "" {
			return path.Join(m.mount, "metadata")
		}
		return path.Join(m.mount, "metadata", storagePath)
	}
	if storagePath == "" {
		return m.mount
	}
	return path.Join(m.mount, storagePath)
}

func (m *VaultSecretsManager) storagePath(resourcePath string) string {
	normalized := resource.NormalizePath(resourcePath)
	rel := strings.TrimPrefix(normalized, "/")
	if normalized == "/" {
		rel = vaultRootResourcePlaceholder
	}
	if m.pathPrefix == "" {
		return rel
	}
	if rel == "" {
		return m.pathPrefix
	}
	return path.Join(m.pathPrefix, rel)
}

func (m *VaultSecretsManager) storagePathFromRel(rel string) string {
	if m.pathPrefix == "" {
		return rel
	}
	if rel == "" {
		return m.pathPrefix
	}
	return path.Join(m.pathPrefix, rel)
}

func (m *VaultSecretsManager) resourcePathFromRel(rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	if rel == vaultRootResourcePlaceholder || rel == "" {
		return "/"
	}
	return "/" + rel
}

func (m *VaultSecretsManager) requestJSON(method, apiPath string, payload any, out any) error {
	m.mu.RLock()
	client := m.client
	address := m.address
	token := m.token
	m.mu.RUnlock()

	if client == nil {
		return errors.New("vault client is not initialized")
	}
	url := strings.TrimSuffix(address, "/") + "/v1/" + strings.TrimPrefix(apiPath, "/")

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			return fs.ErrNotExist
		}
		return fmt.Errorf("vault request failed: %s", strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func stringifySecretData(raw map[string]any) (map[string]string, error) {
	if raw == nil {
		return map[string]string{}, nil
	}
	converted := make(map[string]string, len(raw))
	for key, value := range raw {
		str, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("secret value for %q is not a string", key)
		}
		converted[key] = str
	}
	return converted, nil
}
