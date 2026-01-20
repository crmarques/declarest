package context

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"gopkg.in/yaml.v3"
)

const (
	contextEnvPrefix  = "DECLAREST_CTX_"
	contextEnvNameVar = contextEnvPrefix + "NAME"
)

var (
	contextEnvSetters = map[string]func(*ContextConfig, string) error{
		"managed_server.http.base_url":                            setManagedServerHTTPBaseURL,
		"managed_server.http.openapi":                             setManagedServerHTTPOpenAPI,
		"managed_server.http.default_headers":                     setManagedServerHTTPDefaultHeaders,
		"managed_server.http.auth.oauth2.token_url":               setManagedServerHTTPAuthOAuth2TokenURL,
		"managed_server.http.auth.oauth2.grant_type":              setManagedServerHTTPAuthOAuth2GrantType,
		"managed_server.http.auth.oauth2.client_id":               setManagedServerHTTPAuthOAuth2ClientID,
		"managed_server.http.auth.oauth2.client_secret":           setManagedServerHTTPAuthOAuth2ClientSecret,
		"managed_server.http.auth.oauth2.username":                setManagedServerHTTPAuthOAuth2Username,
		"managed_server.http.auth.oauth2.password":                setManagedServerHTTPAuthOAuth2Password,
		"managed_server.http.auth.oauth2.scope":                   setManagedServerHTTPAuthOAuth2Scope,
		"managed_server.http.auth.oauth2.audience":                setManagedServerHTTPAuthOAuth2Audience,
		"managed_server.http.auth.custom_header.header":           setManagedServerHTTPAuthCustomHeaderHeader,
		"managed_server.http.auth.custom_header.token":            setManagedServerHTTPAuthCustomHeaderToken,
		"managed_server.http.auth.basic_auth.username":            setManagedServerHTTPAuthBasicUsername,
		"managed_server.http.auth.basic_auth.password":            setManagedServerHTTPAuthBasicPassword,
		"managed_server.http.auth.bearer_token.token":             setManagedServerHTTPAuthBearerToken,
		"managed_server.http.tls.ca_cert_file":                    setManagedServerHTTPTLSCACertFile,
		"managed_server.http.tls.client_cert_file":                setManagedServerHTTPTLSClientCertFile,
		"managed_server.http.tls.client_key_file":                 setManagedServerHTTPTLSClientKeyFile,
		"managed_server.http.tls.insecure_skip_verify":            setManagedServerHTTPTLSInsecureSkipVerify,
		"repository.resource_format":                              setRepositoryResourceFormat,
		"repository.git.local.base_dir":                           setRepositoryGitLocalBaseDir,
		"repository.git.remote.url":                               setRepositoryGitRemoteURL,
		"repository.git.remote.branch":                            setRepositoryGitRemoteBranch,
		"repository.git.remote.provider":                          setRepositoryGitRemoteProvider,
		"repository.git.remote.auto_sync":                         setRepositoryGitRemoteAutoSync,
		"repository.git.remote.auth.basic_auth.username":          setRepositoryGitRemoteAuthBasicUsername,
		"repository.git.remote.auth.basic_auth.password":          setRepositoryGitRemoteAuthBasicPassword,
		"repository.git.remote.auth.ssh.user":                     setRepositoryGitRemoteAuthSSHUser,
		"repository.git.remote.auth.ssh.private_key_file":         setRepositoryGitRemoteAuthSSHPrivateKeyFile,
		"repository.git.remote.auth.ssh.passphrase":               setRepositoryGitRemoteAuthSSHPassphrase,
		"repository.git.remote.auth.ssh.known_hosts_file":         setRepositoryGitRemoteAuthSSHKnownHostsFile,
		"repository.git.remote.auth.ssh.insecure_ignore_host_key": setRepositoryGitRemoteAuthSSHInsecureIgnoreHostKey,
		"repository.git.remote.auth.access_key.token":             setRepositoryGitRemoteAuthAccessKeyToken,
		"repository.git.remote.tls.insecure_skip_verify":          setRepositoryGitRemoteTLSInsecureSkipVerify,
		"repository.filesystem.base_dir":                          setRepositoryFilesystemBaseDir,
		"metadata.base_dir":                                       setMetadataBaseDir,
		"secret_store.file.path":                                  setSecretStoreFilePath,
		"secret_store.file.key":                                   setSecretStoreFileKey,
		"secret_store.file.key_file":                              setSecretStoreFileKeyFile,
		"secret_store.file.passphrase":                            setSecretStoreFilePassphrase,
		"secret_store.file.passphrase_file":                       setSecretStoreFilePassphraseFile,
		"secret_store.file.kdf.time":                              setSecretStoreFileKDFTime,
		"secret_store.file.kdf.memory":                            setSecretStoreFileKDFMemory,
		"secret_store.file.kdf.threads":                           setSecretStoreFileKDFThreads,
		"secret_store.vault.address":                              setSecretStoreVaultAddress,
		"secret_store.vault.mount":                                setSecretStoreVaultMount,
		"secret_store.vault.path_prefix":                          setSecretStoreVaultPathPrefix,
		"secret_store.vault.kv_version":                           setSecretStoreVaultKVVersion,
		"secret_store.vault.auth.token":                           setSecretStoreVaultAuthToken,
		"secret_store.vault.auth.password.username":               setSecretStoreVaultAuthPasswordUsername,
		"secret_store.vault.auth.password.password":               setSecretStoreVaultAuthPasswordPassword,
		"secret_store.vault.auth.password.mount":                  setSecretStoreVaultAuthPasswordMount,
		"secret_store.vault.auth.approle.role_id":                 setSecretStoreVaultAuthAppRoleRoleID,
		"secret_store.vault.auth.approle.secret_id":               setSecretStoreVaultAuthAppRoleSecretID,
		"secret_store.vault.auth.approle.mount":                   setSecretStoreVaultAuthAppRoleMount,
		"secret_store.vault.tls.ca_cert_file":                     setSecretStoreVaultTLSCACertFile,
		"secret_store.vault.tls.client_cert_file":                 setSecretStoreVaultTLSClientCertFile,
		"secret_store.vault.tls.client_key_file":                  setSecretStoreVaultTLSClientKeyFile,
		"secret_store.vault.tls.insecure_skip_verify":             setSecretStoreVaultTLSInsecureSkipVerify,
	}
	contextEnvSuffixToPath map[string]string
)

type contextConfigAccessor interface {
	GetContextConfig(name string) (*ContextConfig, bool, error)
}

func init() {
	contextEnvSuffixToPath = make(map[string]string, len(contextEnvSetters))
	for path := range contextEnvSetters {
		contextEnvSuffixToPath[toEnvSuffix(path)] = path
	}
}

func toEnvSuffix(path string) string {
	return strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
}

func contextNameFromEnv() string {
	return strings.TrimSpace(os.Getenv(contextEnvNameVar))
}

func contextOverridesFromEnv() (map[string]string, error) {
	overrides := make(map[string]string)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, contextEnvPrefix) {
			continue
		}
		pair := strings.SplitN(env, "=", 2)
		key := strings.TrimPrefix(pair[0], contextEnvPrefix)
		if key == "" || key == "NAME" {
			continue
		}
		path, ok := contextEnvSuffixToPath[key]
		if !ok {
			return nil, fmt.Errorf("unsupported context override %q", pair[0])
		}
		value := ""
		if len(pair) == 2 {
			value = pair[1]
		}
		overrides[path] = value
	}
	return overrides, nil
}

func LoadContextWithEnv(manager ContextManager) (Context, error) {
	overrides, err := contextOverridesFromEnv()
	if err != nil {
		return Context{}, err
	}
	name := contextNameFromEnv()
	if name != "" {
		return loadContextByName(manager, name, overrides, false)
	}
	return loadDefaultContextWithOverrides(manager, overrides)
}

func loadDefaultContextWithOverrides(manager ContextManager, overrides map[string]string) (Context, error) {
	defaultName, err := manager.GetDefaultContext()
	if err != nil {
		return Context{}, err
	}
	return loadContextByName(manager, defaultName, overrides, true)
}

func loadContextByName(manager ContextManager, name string, overrides map[string]string, requireExists bool) (Context, error) {
	accessor, ok := manager.(contextConfigAccessor)
	if !ok {
		return Context{}, fmt.Errorf("context manager cannot expose stored configuration")
	}
	cfg, exists, err := accessor.GetContextConfig(name)
	if err != nil {
		return Context{}, err
	}
	if !exists {
		if requireExists {
			return Context{}, fmt.Errorf("context %q not found", name)
		}
		if len(overrides) == 0 {
			return Context{}, fmt.Errorf("context %q not found and no %s<attribute> overrides provided", name, contextEnvPrefix)
		}
		cfg = &ContextConfig{}
	} else {
		cfg, err = cloneContextConfig(cfg)
		if err != nil {
			return Context{}, err
		}
	}
	if err := applyContextEnvOverrides(cfg, overrides); err != nil {
		return Context{}, err
	}
	if err := resolveContextEnvPlaceholders(cfg); err != nil {
		return Context{}, fmt.Errorf("failed to resolve environment references for context %q: %w", name, err)
	}
	ctx, err := buildContext(name, cfg)
	if err != nil {
		if !exists {
			return Context{}, fmt.Errorf("failed to build context %q from environment: %w", name, err)
		}
		return Context{}, err
	}
	return ctx, nil
}

func cloneContextConfig(cfg *ContextConfig) (*ContextConfig, error) {
	if cfg == nil {
		return &ContextConfig{}, nil
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var copyCfg ContextConfig
	if err := yaml.Unmarshal(data, &copyCfg); err != nil {
		return nil, err
	}
	return &copyCfg, nil
}

func buildContext(name string, cfg *ContextConfig) (Context, error) {
	recon, err := buildReconcilerFromConfig(cfg)
	if err != nil {
		return Context{}, err
	}
	return Context{Name: name, Reconciler: recon}, nil
}

func applyContextEnvOverrides(cfg *ContextConfig, overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}
	for path, value := range overrides {
		setter, ok := contextEnvSetters[path]
		if !ok {
			return fmt.Errorf("unsupported context attribute %q", path)
		}
		if err := setter(cfg, value); err != nil {
			return fmt.Errorf("failed to apply override %q: %w", path, err)
		}
	}
	return nil
}

func parseBool(value string) (bool, error) {
	result, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, err
	}
	return result, nil
}

func parseInt(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("value is empty")
	}
	return strconv.Atoi(trimmed)
}

func parseUInt32(value string) (uint32, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("value is empty")
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
}

func parseUInt8(value string) (uint8, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("value is empty")
	}
	parsed, err := strconv.ParseUint(trimmed, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(parsed), nil
}

func parseStringMap(value string) (map[string]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var result map[string]string
	if err := yaml.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil, fmt.Errorf("invalid map: %w", err)
	}
	return result, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func setManagedServerHTTPBaseURL(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPCfg(cfg).BaseURL = value
	return nil
}

func setManagedServerHTTPOpenAPI(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPCfg(cfg).OpenAPI = value
	return nil
}

func setManagedServerHTTPDefaultHeaders(cfg *ContextConfig, value string) error {
	headers, err := parseStringMap(value)
	if err != nil {
		return err
	}
	ensureManagedServerHTTPCfg(cfg).DefaultHeaders = headers
	return nil
}

func setManagedServerHTTPAuthOAuth2TokenURL(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).TokenURL = value
	return nil
}

func setManagedServerHTTPAuthOAuth2GrantType(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).GrantType = value
	return nil
}

func setManagedServerHTTPAuthOAuth2ClientID(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).ClientID = value
	return nil
}

func setManagedServerHTTPAuthOAuth2ClientSecret(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).ClientSecret = value
	return nil
}

func setManagedServerHTTPAuthOAuth2Username(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).Username = value
	return nil
}

func setManagedServerHTTPAuthOAuth2Password(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).Password = value
	return nil
}

func setManagedServerHTTPAuthOAuth2Scope(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).Scope = value
	return nil
}

func setManagedServerHTTPAuthOAuth2Audience(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthOAuth2(cfg).Audience = value
	return nil
}

func setManagedServerHTTPAuthCustomHeaderHeader(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthCustomHeader(cfg).Header = value
	return nil
}

func setManagedServerHTTPAuthCustomHeaderToken(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthCustomHeader(cfg).Token = value
	return nil
}

func setManagedServerHTTPAuthBasicUsername(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthBasic(cfg).Username = value
	return nil
}

func setManagedServerHTTPAuthBasicPassword(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthBasic(cfg).Password = value
	return nil
}

func setManagedServerHTTPAuthBearerToken(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPAuthBearer(cfg).Token = value
	return nil
}

func setManagedServerHTTPTLSCACertFile(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPTLSConfig(cfg).CACertFile = value
	return nil
}

func setManagedServerHTTPTLSClientCertFile(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPTLSConfig(cfg).ClientCertFile = value
	return nil
}

func setManagedServerHTTPTLSClientKeyFile(cfg *ContextConfig, value string) error {
	ensureManagedServerHTTPTLSConfig(cfg).ClientKeyFile = value
	return nil
}

func setManagedServerHTTPTLSInsecureSkipVerify(cfg *ContextConfig, value string) error {
	parsed, err := parseBool(value)
	if err != nil {
		return err
	}
	ensureManagedServerHTTPTLSConfig(cfg).InsecureSkipVerify = parsed
	return nil
}

func setRepositoryResourceFormat(cfg *ContextConfig, value string) error {
	ensureRepositoryConfig(cfg).ResourceFormat = value
	return nil
}

func setRepositoryGitLocalBaseDir(cfg *ContextConfig, value string) error {
	ensureRepositoryGitLocalConfig(cfg).BaseDir = value
	return nil
}

func setRepositoryGitRemoteURL(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteConfig(cfg).URL = value
	return nil
}

func setRepositoryGitRemoteBranch(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteConfig(cfg).Branch = value
	return nil
}

func setRepositoryGitRemoteProvider(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteConfig(cfg).Provider = value
	return nil
}

func setRepositoryGitRemoteAutoSync(cfg *ContextConfig, value string) error {
	parsed, err := parseBool(value)
	if err != nil {
		return err
	}
	ensureRepositoryGitRemoteConfig(cfg).AutoSync = boolPtr(parsed)
	return nil
}

func setRepositoryGitRemoteAuthBasicUsername(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthBasic(cfg).Username = value
	return nil
}

func setRepositoryGitRemoteAuthBasicPassword(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthBasic(cfg).Password = value
	return nil
}

func setRepositoryGitRemoteAuthSSHUser(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthSSH(cfg).User = value
	return nil
}

func setRepositoryGitRemoteAuthSSHPrivateKeyFile(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthSSH(cfg).PrivateKeyFile = value
	return nil
}

func setRepositoryGitRemoteAuthSSHPassphrase(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthSSH(cfg).Passphrase = value
	return nil
}

func setRepositoryGitRemoteAuthSSHKnownHostsFile(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthSSH(cfg).KnownHostsFile = value
	return nil
}

func setRepositoryGitRemoteAuthSSHInsecureIgnoreHostKey(cfg *ContextConfig, value string) error {
	parsed, err := parseBool(value)
	if err != nil {
		return err
	}
	ensureRepositoryGitRemoteAuthSSH(cfg).InsecureIgnoreHostKey = parsed
	return nil
}

func setRepositoryGitRemoteAuthAccessKeyToken(cfg *ContextConfig, value string) error {
	ensureRepositoryGitRemoteAuthAccessKey(cfg).Token = value
	return nil
}

func setRepositoryGitRemoteTLSInsecureSkipVerify(cfg *ContextConfig, value string) error {
	parsed, err := parseBool(value)
	if err != nil {
		return err
	}
	ensureRepositoryGitRemoteTLSConfig(cfg).InsecureSkipVerify = parsed
	return nil
}

func setRepositoryFilesystemBaseDir(cfg *ContextConfig, value string) error {
	ensureRepositoryFilesystemConfig(cfg).BaseDir = value
	return nil
}

func setMetadataBaseDir(cfg *ContextConfig, value string) error {
	if cfg.Metadata == nil {
		cfg.Metadata = &MetadataConfig{}
	}
	cfg.Metadata.BaseDir = value
	return nil
}

func setSecretStoreFilePath(cfg *ContextConfig, value string) error {
	ensureFileSecretsManagerConfig(cfg).Path = value
	return nil
}

func setSecretStoreFileKey(cfg *ContextConfig, value string) error {
	ensureFileSecretsManagerConfig(cfg).Key = value
	return nil
}

func setSecretStoreFileKeyFile(cfg *ContextConfig, value string) error {
	ensureFileSecretsManagerConfig(cfg).KeyFile = value
	return nil
}

func setSecretStoreFilePassphrase(cfg *ContextConfig, value string) error {
	ensureFileSecretsManagerConfig(cfg).Passphrase = value
	return nil
}

func setSecretStoreFilePassphraseFile(cfg *ContextConfig, value string) error {
	ensureFileSecretsManagerConfig(cfg).PassphraseFile = value
	return nil
}

func setSecretStoreFileKDFTime(cfg *ContextConfig, value string) error {
	parsed, err := parseUInt32(value)
	if err != nil {
		return err
	}
	ensureFileSecretsManagerKDFConfig(cfg).Time = parsed
	return nil
}

func setSecretStoreFileKDFMemory(cfg *ContextConfig, value string) error {
	parsed, err := parseUInt32(value)
	if err != nil {
		return err
	}
	ensureFileSecretsManagerKDFConfig(cfg).Memory = parsed
	return nil
}

func setSecretStoreFileKDFThreads(cfg *ContextConfig, value string) error {
	parsed, err := parseUInt8(value)
	if err != nil {
		return err
	}
	ensureFileSecretsManagerKDFConfig(cfg).Threads = parsed
	return nil
}

func setSecretStoreVaultAddress(cfg *ContextConfig, value string) error {
	ensureVaultSecretsManagerConfig(cfg).Address = value
	return nil
}

func setSecretStoreVaultMount(cfg *ContextConfig, value string) error {
	ensureVaultSecretsManagerConfig(cfg).Mount = value
	return nil
}

func setSecretStoreVaultPathPrefix(cfg *ContextConfig, value string) error {
	ensureVaultSecretsManagerConfig(cfg).PathPrefix = value
	return nil
}

func setSecretStoreVaultKVVersion(cfg *ContextConfig, value string) error {
	parsed, err := parseInt(value)
	if err != nil {
		return err
	}
	ensureVaultSecretsManagerConfig(cfg).KVVersion = parsed
	return nil
}

func setSecretStoreVaultAuthToken(cfg *ContextConfig, value string) error {
	ensureVaultAuthConfig(cfg).Token = value
	return nil
}

func setSecretStoreVaultAuthPasswordUsername(cfg *ContextConfig, value string) error {
	ensureVaultAuthPasswordConfig(cfg).Username = value
	return nil
}

func setSecretStoreVaultAuthPasswordPassword(cfg *ContextConfig, value string) error {
	ensureVaultAuthPasswordConfig(cfg).Password = value
	return nil
}

func setSecretStoreVaultAuthPasswordMount(cfg *ContextConfig, value string) error {
	ensureVaultAuthPasswordConfig(cfg).Mount = value
	return nil
}

func setSecretStoreVaultAuthAppRoleRoleID(cfg *ContextConfig, value string) error {
	ensureVaultAuthAppRoleConfig(cfg).RoleID = value
	return nil
}

func setSecretStoreVaultAuthAppRoleSecretID(cfg *ContextConfig, value string) error {
	ensureVaultAuthAppRoleConfig(cfg).SecretID = value
	return nil
}

func setSecretStoreVaultAuthAppRoleMount(cfg *ContextConfig, value string) error {
	ensureVaultAuthAppRoleConfig(cfg).Mount = value
	return nil
}

func setSecretStoreVaultTLSCACertFile(cfg *ContextConfig, value string) error {
	ensureVaultTLSConfig(cfg).CACertFile = value
	return nil
}

func setSecretStoreVaultTLSClientCertFile(cfg *ContextConfig, value string) error {
	ensureVaultTLSConfig(cfg).ClientCertFile = value
	return nil
}

func setSecretStoreVaultTLSClientKeyFile(cfg *ContextConfig, value string) error {
	ensureVaultTLSConfig(cfg).ClientKeyFile = value
	return nil
}

func setSecretStoreVaultTLSInsecureSkipVerify(cfg *ContextConfig, value string) error {
	parsed, err := parseBool(value)
	if err != nil {
		return err
	}
	ensureVaultTLSConfig(cfg).InsecureSkipVerify = parsed
	return nil
}

func ensureManagedServerConfig(cfg *ContextConfig) *ManagedServerConfig {
	if cfg.ManagedServer == nil {
		cfg.ManagedServer = &ManagedServerConfig{}
	}
	return cfg.ManagedServer
}

func ensureManagedServerHTTPCfg(cfg *ContextConfig) *managedserver.HTTPResourceServerConfig {
	ms := ensureManagedServerConfig(cfg)
	if ms.HTTP == nil {
		ms.HTTP = &managedserver.HTTPResourceServerConfig{}
	}
	return ms.HTTP
}

func ensureManagedServerHTTPAuthConfig(cfg *ContextConfig) *managedserver.HTTPResourceServerAuthConfig {
	httpCfg := ensureManagedServerHTTPCfg(cfg)
	if httpCfg.Auth == nil {
		httpCfg.Auth = &managedserver.HTTPResourceServerAuthConfig{}
	}
	return httpCfg.Auth
}

func ensureManagedServerHTTPAuthOAuth2(cfg *ContextConfig) *managedserver.HTTPResourceServerOAuth2Config {
	authCfg := ensureManagedServerHTTPAuthConfig(cfg)
	if authCfg.OAuth2 == nil {
		authCfg.OAuth2 = &managedserver.HTTPResourceServerOAuth2Config{}
	}
	return authCfg.OAuth2
}

func ensureManagedServerHTTPAuthCustomHeader(cfg *ContextConfig) *managedserver.HTTPResourceServerCustomHeaderConfig {
	authCfg := ensureManagedServerHTTPAuthConfig(cfg)
	if authCfg.CustomHeader == nil {
		authCfg.CustomHeader = &managedserver.HTTPResourceServerCustomHeaderConfig{}
	}
	return authCfg.CustomHeader
}

func ensureManagedServerHTTPAuthBasic(cfg *ContextConfig) *managedserver.HTTPResourceServerBasicAuthConfig {
	authCfg := ensureManagedServerHTTPAuthConfig(cfg)
	if authCfg.BasicAuth == nil {
		authCfg.BasicAuth = &managedserver.HTTPResourceServerBasicAuthConfig{}
	}
	return authCfg.BasicAuth
}

func ensureManagedServerHTTPAuthBearer(cfg *ContextConfig) *managedserver.HTTPResourceServerBearerTokenConfig {
	authCfg := ensureManagedServerHTTPAuthConfig(cfg)
	if authCfg.BearerToken == nil {
		authCfg.BearerToken = &managedserver.HTTPResourceServerBearerTokenConfig{}
	}
	return authCfg.BearerToken
}

func ensureManagedServerHTTPTLSConfig(cfg *ContextConfig) *managedserver.HTTPResourceServerTLSConfig {
	httpCfg := ensureManagedServerHTTPCfg(cfg)
	if httpCfg.TLS == nil {
		httpCfg.TLS = &managedserver.HTTPResourceServerTLSConfig{}
	}
	return httpCfg.TLS
}

func ensureRepositoryConfig(cfg *ContextConfig) *RepositoryConfig {
	if cfg.Repository == nil {
		cfg.Repository = &RepositoryConfig{}
	}
	return cfg.Repository
}

func ensureRepositoryGitConfig(cfg *ContextConfig) *repository.GitResourceRepositoryConfig {
	repo := ensureRepositoryConfig(cfg)
	if repo.Git == nil {
		repo.Git = &repository.GitResourceRepositoryConfig{}
	}
	return repo.Git
}

func ensureRepositoryGitLocalConfig(cfg *ContextConfig) *repository.GitResourceRepositoryLocalConfig {
	gitCfg := ensureRepositoryGitConfig(cfg)
	if gitCfg.Local == nil {
		gitCfg.Local = &repository.GitResourceRepositoryLocalConfig{}
	}
	return gitCfg.Local
}

func ensureRepositoryGitRemoteConfig(cfg *ContextConfig) *repository.GitResourceRepositoryRemoteConfig {
	gitCfg := ensureRepositoryGitConfig(cfg)
	if gitCfg.Remote == nil {
		gitCfg.Remote = &repository.GitResourceRepositoryRemoteConfig{}
	}
	return gitCfg.Remote
}

func ensureRepositoryGitRemoteAuthConfig(cfg *ContextConfig) *repository.GitResourceRepositoryRemoteAuthConfig {
	remote := ensureRepositoryGitRemoteConfig(cfg)
	if remote.Auth == nil {
		remote.Auth = &repository.GitResourceRepositoryRemoteAuthConfig{}
	}
	return remote.Auth
}

func ensureRepositoryGitRemoteAuthBasic(cfg *ContextConfig) *repository.GitResourceRepositoryBasicAuthConfig {
	auth := ensureRepositoryGitRemoteAuthConfig(cfg)
	if auth.BasicAuth == nil {
		auth.BasicAuth = &repository.GitResourceRepositoryBasicAuthConfig{}
	}
	return auth.BasicAuth
}

func ensureRepositoryGitRemoteAuthSSH(cfg *ContextConfig) *repository.GitResourceRepositorySSHAuthConfig {
	auth := ensureRepositoryGitRemoteAuthConfig(cfg)
	if auth.SSH == nil {
		auth.SSH = &repository.GitResourceRepositorySSHAuthConfig{}
	}
	return auth.SSH
}

func ensureRepositoryGitRemoteAuthAccessKey(cfg *ContextConfig) *repository.GitResourceRepositoryAccessKeyConfig {
	auth := ensureRepositoryGitRemoteAuthConfig(cfg)
	if auth.AccessKey == nil {
		auth.AccessKey = &repository.GitResourceRepositoryAccessKeyConfig{}
	}
	return auth.AccessKey
}

func ensureRepositoryGitRemoteTLSConfig(cfg *ContextConfig) *repository.GitResourceRepositoryRemoteTLSConfig {
	remote := ensureRepositoryGitRemoteConfig(cfg)
	if remote.TLS == nil {
		remote.TLS = &repository.GitResourceRepositoryRemoteTLSConfig{}
	}
	return remote.TLS
}

func ensureRepositoryFilesystemConfig(cfg *ContextConfig) *repository.FileSystemResourceRepositoryConfig {
	repo := ensureRepositoryConfig(cfg)
	if repo.Filesystem == nil {
		repo.Filesystem = &repository.FileSystemResourceRepositoryConfig{}
	}
	return repo.Filesystem
}

func ensureSecretManagerConfig(cfg *ContextConfig) *secrets.SecretsManagerConfig {
	if cfg.SecretManager == nil {
		cfg.SecretManager = &secrets.SecretsManagerConfig{}
	}
	return cfg.SecretManager
}

func ensureFileSecretsManagerConfig(cfg *ContextConfig) *secrets.FileSecretsManagerConfig {
	secret := ensureSecretManagerConfig(cfg)
	if secret.File == nil {
		secret.File = &secrets.FileSecretsManagerConfig{}
	}
	return secret.File
}

func ensureFileSecretsManagerKDFConfig(cfg *ContextConfig) *secrets.FileSecretsManagerKDFConfig {
	file := ensureFileSecretsManagerConfig(cfg)
	if file.KDF == nil {
		file.KDF = &secrets.FileSecretsManagerKDFConfig{}
	}
	return file.KDF
}

func ensureVaultSecretsManagerConfig(cfg *ContextConfig) *secrets.VaultSecretsManagerConfig {
	secret := ensureSecretManagerConfig(cfg)
	if secret.Vault == nil {
		secret.Vault = &secrets.VaultSecretsManagerConfig{}
	}
	return secret.Vault
}

func ensureVaultAuthConfig(cfg *ContextConfig) *secrets.VaultSecretsManagerAuthConfig {
	vault := ensureVaultSecretsManagerConfig(cfg)
	if vault.Auth == nil {
		vault.Auth = &secrets.VaultSecretsManagerAuthConfig{}
	}
	return vault.Auth
}

func ensureVaultAuthPasswordConfig(cfg *ContextConfig) *secrets.VaultSecretsManagerPasswordAuthConfig {
	auth := ensureVaultAuthConfig(cfg)
	if auth.Password == nil {
		auth.Password = &secrets.VaultSecretsManagerPasswordAuthConfig{}
	}
	return auth.Password
}

func ensureVaultAuthAppRoleConfig(cfg *ContextConfig) *secrets.VaultSecretsManagerAppRoleAuthConfig {
	auth := ensureVaultAuthConfig(cfg)
	if auth.AppRole == nil {
		auth.AppRole = &secrets.VaultSecretsManagerAppRoleAuthConfig{}
	}
	return auth.AppRole
}

func ensureVaultTLSConfig(cfg *ContextConfig) *secrets.VaultSecretsManagerTLSConfig {
	vault := ensureVaultSecretsManagerConfig(cfg)
	if vault.TLS == nil {
		vault.TLS = &secrets.VaultSecretsManagerTLSConfig{}
	}
	return vault.TLS
}
