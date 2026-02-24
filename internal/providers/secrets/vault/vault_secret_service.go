package vault

import (
	"context"
	"net/http"
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
