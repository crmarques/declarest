package vault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

const (
	defaultVaultTimeout = 30 * time.Second
	defaultVaultMount   = "secret"
	defaultVaultKV      = 2
)

var _ secretdomain.SecretProvider = (*VaultSecretService)(nil)

type vaultAuthMode int

const (
	vaultAuthToken vaultAuthMode = iota
	vaultAuthUserPass
	vaultAuthAppRole
)

type VaultSecretService struct {
	address    string
	mount      string
	pathPrefix string
	kvVersion  int
	auth       vaultAuthConfig
	client     *http.Client

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

type vaultResponse struct {
	Data   map[string]any `json:"data"`
	Errors []string       `json:"errors"`
	Auth   *vaultAuthInfo `json:"auth"`
}

type vaultAuthInfo struct {
	ClientToken string `json:"client_token"`
}

func NewVaultSecretService(cfg config.VaultSecretStore) (*VaultSecretService, error) {
	address, err := normalizeVaultAddress(cfg.Address)
	if err != nil {
		return nil, err
	}

	mount, err := normalizeVaultPath(cfg.Mount, true)
	if err != nil {
		return nil, validationError("secret-store.vault.mount is invalid", err)
	}
	if mount == "" {
		mount = defaultVaultMount
	}

	pathPrefix, err := normalizeVaultPath(cfg.PathPrefix, true)
	if err != nil {
		return nil, validationError("secret-store.vault.path-prefix is invalid", err)
	}

	kvVersion := cfg.KVVersion
	if kvVersion == 0 {
		kvVersion = defaultVaultKV
	}
	if kvVersion != 1 && kvVersion != 2 {
		return nil, validationError("secret-store.vault.kv-version must be 1 or 2", nil)
	}

	if cfg.Auth == nil {
		return nil, validationError("secret-store.vault.auth is required", nil)
	}

	auth, err := buildVaultAuthConfig(*cfg.Auth)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	service := &VaultSecretService{
		address:    address,
		mount:      mount,
		pathPrefix: pathPrefix,
		kvVersion:  kvVersion,
		auth:       auth,
		client: &http.Client{
			Timeout:   defaultVaultTimeout,
			Transport: transport,
		},
	}

	if auth.mode == vaultAuthToken {
		service.token = auth.token
	}

	return service, nil
}

func (s *VaultSecretService) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked(ctx)
}

func (s *VaultSecretService) Store(ctx context.Context, key string, value string) error {
	normalizedKey, err := normalizeSecretKey(key)
	if err != nil {
		return err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return err
	}

	payload := map[string]any{}
	if s.kvVersion == 2 {
		payload["data"] = map[string]string{"value": value}
	} else {
		payload["value"] = value
	}

	endpoint := s.writeEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	return mapVaultStatus(status, response, false, "")
}

func (s *VaultSecretService) Get(ctx context.Context, key string) (string, error) {
	normalizedKey, err := normalizeSecretKey(key)
	if err != nil {
		return "", err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return "", err
	}

	endpoint := s.readEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	if err := mapVaultStatus(status, response, false, "secret key not found"); err != nil {
		return "", err
	}

	value, err := s.extractValue(response)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *VaultSecretService) Delete(ctx context.Context, key string) error {
	normalizedKey, err := normalizeSecretKey(key)
	if err != nil {
		return err
	}

	if err := s.ensureInitialized(ctx); err != nil {
		return err
	}

	endpoint := s.deleteEndpoint(normalizedKey)
	response, status, err := s.request(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	// Delete is idempotent. Missing keys are treated as success.
	if status == http.StatusNotFound {
		return nil
	}

	return mapVaultStatus(status, response, false, "")
}

func (s *VaultSecretService) List(ctx context.Context) ([]string, error) {
	if err := s.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	endpoint := s.listEndpoint("")
	response, status, err := s.request(ctx, "LIST", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusBadRequest {
		fallbackEndpoint := endpoint + "?list=true"
		response, status, err = s.request(ctx, http.MethodGet, fallbackEndpoint, nil)
		if err != nil {
			return nil, err
		}
	}
	if err := mapVaultStatus(status, response, true, ""); err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return []string{}, nil
	}

	rawKeys, found := response.Data["keys"]
	if !found {
		return []string{}, nil
	}

	typedKeys, ok := rawKeys.([]any)
	if !ok {
		return nil, internalError("vault list response payload is invalid", nil)
	}

	keys := make([]string, 0, len(typedKeys))
	for _, item := range typedKeys {
		key, ok := item.(string)
		if !ok {
			return nil, internalError("vault list response payload is invalid", nil)
		}
		key = strings.TrimSpace(strings.TrimSuffix(key, "/"))
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	return keys, nil
}

func (s *VaultSecretService) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return s.Store(ctx, key, secretValue)
	})
}

func (s *VaultSecretService) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return s.Get(ctx, key)
	})
}

func (s *VaultSecretService) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (s *VaultSecretService) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

func (s *VaultSecretService) ensureInitialized(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.initLocked(ctx)
}

func (s *VaultSecretService) initLocked(ctx context.Context) error {
	if s == nil {
		return validationError("vault secret service must not be nil", nil)
	}
	if s.initialized {
		return nil
	}

	switch s.auth.mode {
	case vaultAuthToken:
		if strings.TrimSpace(s.auth.token) == "" {
			return authError("vault token auth requires token", nil)
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
		return validationError("vault auth mode is invalid", nil)
	}

	if strings.TrimSpace(s.token) == "" {
		return authError("vault authentication did not return a token", nil)
	}

	s.initialized = true
	return nil
}

func (s *VaultSecretService) loginUserPass(ctx context.Context) error {
	credentials := s.auth.userPass
	if credentials == nil {
		return validationError("vault userpass auth configuration is invalid", nil)
	}

	mount, err := normalizeVaultPath(credentials.Mount, true)
	if err != nil {
		return validationError("secret-store.vault.auth.password.mount is invalid", err)
	}
	if mount == "" {
		mount = "userpass"
	}

	username := strings.TrimSpace(credentials.Username)
	password := strings.TrimSpace(credentials.Password)
	if username == "" || password == "" {
		return validationError("secret-store.vault.auth.password requires username and password", nil)
	}

	endpoint := buildEndpoint("auth", mount, "login", username)
	payload := map[string]string{"password": password}

	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if err := mapVaultStatus(status, response, false, ""); err != nil {
		return err
	}
	if response.Auth == nil || strings.TrimSpace(response.Auth.ClientToken) == "" {
		return authError("vault authentication response did not include a client token", nil)
	}

	s.token = strings.TrimSpace(response.Auth.ClientToken)
	return nil
}

func (s *VaultSecretService) loginAppRole(ctx context.Context) error {
	credentials := s.auth.appRole
	if credentials == nil {
		return validationError("vault approle auth configuration is invalid", nil)
	}

	mount, err := normalizeVaultPath(credentials.Mount, true)
	if err != nil {
		return validationError("secret-store.vault.auth.approle.mount is invalid", err)
	}
	if mount == "" {
		mount = "approle"
	}

	roleID := strings.TrimSpace(credentials.RoleID)
	secretID := strings.TrimSpace(credentials.SecretID)
	if roleID == "" || secretID == "" {
		return validationError("secret-store.vault.auth.approle requires role-id and secret-id", nil)
	}

	endpoint := buildEndpoint("auth", mount, "login")
	payload := map[string]string{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	response, status, err := s.request(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return err
	}
	if err := mapVaultStatus(status, response, false, ""); err != nil {
		return err
	}
	if response.Auth == nil || strings.TrimSpace(response.Auth.ClientToken) == "" {
		return authError("vault authentication response did not include a client token", nil)
	}

	s.token = strings.TrimSpace(response.Auth.ClientToken)
	return nil
}

func (s *VaultSecretService) request(
	ctx context.Context,
	method string,
	endpoint string,
	payload any,
) (vaultResponse, int, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return vaultResponse{}, 0, internalError("failed to encode vault request payload", err)
		}
		body = strings.NewReader(string(encoded))
	}

	requestURL := s.address + endpoint
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return vaultResponse{}, 0, internalError("failed to build vault request", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := strings.TrimSpace(s.token); token != "" {
		req.Header.Set("X-Vault-Token", token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return vaultResponse{}, 0, transportError("vault request failed", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return vaultResponse{}, 0, transportError("failed to read vault response body", err)
	}

	if len(data) == 0 {
		return vaultResponse{}, resp.StatusCode, nil
	}

	var decoded vaultResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return vaultResponse{}, 0, transportError("failed to decode vault response body", err)
	}

	return decoded, resp.StatusCode, nil
}

func (s *VaultSecretService) readEndpoint(key string) string {
	fullPath := s.fullSecretPath(key)
	if s.kvVersion == 2 {
		return buildEndpoint(s.mount, "data", fullPath)
	}
	return buildEndpoint(s.mount, fullPath)
}

func (s *VaultSecretService) writeEndpoint(key string) string {
	return s.readEndpoint(key)
}

func (s *VaultSecretService) deleteEndpoint(key string) string {
	return s.readEndpoint(key)
}

func (s *VaultSecretService) listEndpoint(key string) string {
	fullPath := s.fullSecretPath(key)
	if s.kvVersion == 2 {
		return buildEndpoint(s.mount, "metadata", fullPath)
	}
	return buildEndpoint(s.mount, fullPath)
}

func (s *VaultSecretService) fullSecretPath(key string) string {
	switch {
	case s.pathPrefix == "":
		return key
	case key == "":
		return s.pathPrefix
	default:
		return s.pathPrefix + "/" + key
	}
}

func (s *VaultSecretService) extractValue(response vaultResponse) (string, error) {
	if response.Data == nil {
		return "", internalError("vault response missing secret payload", nil)
	}

	target := response.Data
	if s.kvVersion == 2 {
		rawData, found := response.Data["data"]
		if !found {
			return "", internalError("vault response missing kv-v2 data payload", nil)
		}
		typedData, ok := rawData.(map[string]any)
		if !ok {
			return "", internalError("vault response has invalid kv-v2 data payload", nil)
		}
		target = typedData
	}

	rawValue, found := target["value"]
	if !found {
		return "", notFoundError("secret key not found")
	}

	value, ok := rawValue.(string)
	if !ok {
		return "", internalError("vault secret value is not a string", nil)
	}
	return value, nil
}

func mapVaultStatus(status int, response vaultResponse, allowNotFound bool, notFoundMessage string) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusNotFound:
		if allowNotFound {
			return nil
		}
		message := notFoundMessage
		if message == "" {
			message = "vault resource not found"
		}
		return notFoundError(message)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return authError(firstVaultError(response, "vault authentication failed"), nil)
	case status >= 500:
		return transportError(firstVaultError(response, "vault service is unavailable"), nil)
	default:
		return validationError(firstVaultError(response, "vault request failed"), nil)
	}
}

func firstVaultError(response vaultResponse, fallback string) string {
	for _, message := range response.Errors {
		trimmed := strings.TrimSpace(message)
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func buildVaultAuthConfig(cfg config.VaultAuth) (vaultAuthConfig, error) {
	setCount := countSet(
		strings.TrimSpace(cfg.Token) != "",
		cfg.Password != nil,
		cfg.AppRole != nil,
	)
	if setCount != 1 {
		return vaultAuthConfig{}, validationError("secret-store.vault.auth must define exactly one of token, password, approle", nil)
	}

	if strings.TrimSpace(cfg.Token) != "" {
		return vaultAuthConfig{
			mode:  vaultAuthToken,
			token: strings.TrimSpace(cfg.Token),
		}, nil
	}

	if cfg.Password != nil {
		if strings.TrimSpace(cfg.Password.Username) == "" || strings.TrimSpace(cfg.Password.Password) == "" {
			return vaultAuthConfig{}, validationError("secret-store.vault.auth.password requires username and password", nil)
		}
		copied := *cfg.Password
		return vaultAuthConfig{
			mode:     vaultAuthUserPass,
			userPass: &copied,
		}, nil
	}

	if cfg.AppRole != nil {
		if strings.TrimSpace(cfg.AppRole.RoleID) == "" || strings.TrimSpace(cfg.AppRole.SecretID) == "" {
			return vaultAuthConfig{}, validationError("secret-store.vault.auth.approle requires role-id and secret-id", nil)
		}
		copied := *cfg.AppRole
		return vaultAuthConfig{
			mode:    vaultAuthAppRole,
			appRole: &copied,
		}, nil
	}

	return vaultAuthConfig{}, validationError("secret-store.vault.auth is invalid", nil)
}

func buildTLSConfig(tlsSettings *config.TLS) (*tls.Config, error) {
	if tlsSettings == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tlsSettings.InsecureSkipVerify,
	}

	if strings.TrimSpace(tlsSettings.CACertFile) != "" {
		caBytes, err := os.ReadFile(tlsSettings.CACertFile)
		if err != nil {
			return nil, validationError("secret-store.vault.tls.ca-cert-file could not be read", err)
		}

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caBytes); !ok {
			return nil, validationError("secret-store.vault.tls.ca-cert-file is not valid PEM", nil)
		}
		tlsConfig.RootCAs = pool
	}

	clientCertFile := strings.TrimSpace(tlsSettings.ClientCertFile)
	clientKeyFile := strings.TrimSpace(tlsSettings.ClientKeyFile)
	if (clientCertFile == "") != (clientKeyFile == "") {
		return nil, validationError("secret-store.vault.tls requires both client-cert-file and client-key-file", nil)
	}

	if clientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, validationError("secret-store.vault.tls client certificate pair is invalid", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return tlsConfig, nil
}

func normalizeVaultAddress(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", validationError("secret-store.vault.address is required", nil)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", validationError("secret-store.vault.address is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", validationError("secret-store.vault.address must use http or https", nil)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", validationError("secret-store.vault.address host is required", nil)
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeVaultPath(value string, allowEmpty bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		if allowEmpty {
			return "", nil
		}
		return "", validationError("vault path must not be empty", nil)
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", validationError("vault path contains invalid segments", nil)
		}
	}

	return strings.Join(parts, "/"), nil
}

func normalizeSecretKey(key string) (string, error) {
	return normalizeVaultPath(key, false)
}

func buildEndpoint(parts ...string) string {
	encoded := make([]string, 0, len(parts)+1)
	encoded = append(encoded, "v1")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		for _, segment := range strings.Split(part, "/") {
			if segment == "" {
				continue
			}
			encoded = append(encoded, url.PathEscape(segment))
		}
	}

	return "/" + strings.Join(encoded, "/")
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

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func authError(message string, cause error) error {
	return faults.NewTypedError(faults.AuthError, message, cause)
}

func transportError(message string, cause error) error {
	return faults.NewTypedError(faults.TransportError, message, cause)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
