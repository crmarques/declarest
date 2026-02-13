package context

import (
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

func NormalizeContextConfig(cfg *ContextConfig) *ContextConfig {
	if cfg == nil {
		return &ContextConfig{}
	}
	normalized := *cfg
	normalized.Repository = normalizeRepositoryConfig(cfg.Repository)
	normalized.ManagedServer = normalizeManagedServerConfig(cfg.ManagedServer)
	normalized.SecretManager = normalizeSecretsConfig(cfg.SecretManager)
	return &normalized
}

func normalizeRepositoryConfig(cfg *RepositoryConfig) *RepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.ResourceFormat = normalizeResourceFormatValue(cfg.ResourceFormat)
	normalized.Git = normalizeGitRepositoryConfig(cfg.Git)
	normalized.Filesystem = normalizeFilesystemRepositoryConfig(cfg.Filesystem)
	if normalized.ResourceFormat == "" && normalized.Git == nil && normalized.Filesystem == nil {
		return nil
	}
	return &normalized
}

func normalizeResourceFormatValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := repository.ParseResourceFormat(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed == repository.ResourceFormatJSON {
		return ""
	}
	return string(parsed)
}

func normalizeGitRepositoryConfig(cfg *repository.GitResourceRepositoryConfig) *repository.GitResourceRepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Local = normalizeGitLocalConfig(cfg.Local)
	normalized.Remote = normalizeGitRemoteConfig(cfg.Remote)
	if normalized.Local == nil && normalized.Remote == nil {
		return nil
	}
	return &normalized
}

func normalizeGitLocalConfig(cfg *repository.GitResourceRepositoryLocalConfig) *repository.GitResourceRepositoryLocalConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseDir = strings.TrimSpace(cfg.BaseDir)
	if normalized.BaseDir == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteConfig(cfg *repository.GitResourceRepositoryRemoteConfig) *repository.GitResourceRepositoryRemoteConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.URL = strings.TrimSpace(cfg.URL)
	normalized.Branch = strings.TrimSpace(cfg.Branch)
	normalized.Provider = strings.TrimSpace(cfg.Provider)
	if normalized.AutoSync != nil && *normalized.AutoSync {
		normalized.AutoSync = nil
	}
	normalized.Auth = normalizeGitRemoteAuthConfig(cfg.Auth)
	normalized.TLS = normalizeGitRemoteTLSConfig(cfg.TLS)
	if normalized.URL == "" && normalized.Branch == "" && normalized.Provider == "" &&
		normalized.AutoSync == nil && normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteAuthConfig(cfg *repository.GitResourceRepositoryRemoteAuthConfig) *repository.GitResourceRepositoryRemoteAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BasicAuth = normalizeGitRemoteBasicAuth(cfg.BasicAuth)
	normalized.SSH = normalizeGitRemoteSSH(cfg.SSH)
	normalized.AccessKey = normalizeGitRemoteAccessKey(cfg.AccessKey)
	if normalized.BasicAuth == nil && normalized.SSH == nil && normalized.AccessKey == nil {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteBasicAuth(cfg *repository.GitResourceRepositoryBasicAuthConfig) *repository.GitResourceRepositoryBasicAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	if normalized.Username == "" && normalized.Password == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteSSH(cfg *repository.GitResourceRepositorySSHAuthConfig) *repository.GitResourceRepositorySSHAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.User = strings.TrimSpace(cfg.User)
	normalized.PrivateKeyFile = strings.TrimSpace(cfg.PrivateKeyFile)
	normalized.Passphrase = strings.TrimSpace(cfg.Passphrase)
	normalized.KnownHostsFile = strings.TrimSpace(cfg.KnownHostsFile)
	if normalized.User == "" && normalized.PrivateKeyFile == "" && normalized.Passphrase == "" &&
		normalized.KnownHostsFile == "" && !normalized.InsecureIgnoreHostKey {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteAccessKey(cfg *repository.GitResourceRepositoryAccessKeyConfig) *repository.GitResourceRepositoryAccessKeyConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteTLSConfig(cfg *repository.GitResourceRepositoryRemoteTLSConfig) *repository.GitResourceRepositoryRemoteTLSConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	if !normalized.InsecureSkipVerify {
		return nil
	}
	return &normalized
}

func normalizeFilesystemRepositoryConfig(cfg *repository.FileSystemResourceRepositoryConfig) *repository.FileSystemResourceRepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseDir = strings.TrimSpace(cfg.BaseDir)
	if normalized.BaseDir == "" {
		return nil
	}
	return &normalized
}

func normalizeManagedServerConfig(cfg *ManagedServerConfig) *ManagedServerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.HTTP = normalizeHTTPServerConfig(cfg.HTTP)
	if normalized.HTTP == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPServerConfig(cfg *managedserver.HTTPResourceServerConfig) *managedserver.HTTPResourceServerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseURL = strings.TrimSpace(cfg.BaseURL)
	normalized.OpenAPI = strings.TrimSpace(cfg.OpenAPI)
	if len(cfg.DefaultHeaders) == 0 {
		normalized.DefaultHeaders = nil
	} else {
		normalized.DefaultHeaders = copyStringMap(cfg.DefaultHeaders)
	}
	normalized.Auth = normalizeHTTPAuthConfig(cfg.Auth)
	normalized.TLS = normalizeHTTPTLSConfig(cfg.TLS)
	if normalized.BaseURL == "" && normalized.OpenAPI == "" && len(normalized.DefaultHeaders) == 0 &&
		normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPAuthConfig(cfg *managedserver.HTTPResourceServerAuthConfig) *managedserver.HTTPResourceServerAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.OAuth2 = normalizeHTTPOAuth2Config(cfg.OAuth2)
	normalized.BasicAuth = normalizeHTTPBasicAuthConfig(cfg.BasicAuth)
	normalized.BearerToken = normalizeHTTPBearerConfig(cfg.BearerToken)
	normalized.CustomHeader = normalizeHTTPCustomHeaderConfig(cfg.CustomHeader)
	if normalized.OAuth2 == nil && normalized.BasicAuth == nil &&
		normalized.BearerToken == nil && normalized.CustomHeader == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPOAuth2Config(cfg *managedserver.HTTPResourceServerOAuth2Config) *managedserver.HTTPResourceServerOAuth2Config {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.TokenURL = strings.TrimSpace(cfg.TokenURL)
	normalized.GrantType = strings.TrimSpace(cfg.GrantType)
	normalized.ClientID = strings.TrimSpace(cfg.ClientID)
	normalized.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	normalized.Scope = strings.TrimSpace(cfg.Scope)
	normalized.Audience = strings.TrimSpace(cfg.Audience)
	if normalized.TokenURL == "" && normalized.GrantType == "" && normalized.ClientID == "" &&
		normalized.ClientSecret == "" && normalized.Username == "" && normalized.Password == "" &&
		normalized.Scope == "" && normalized.Audience == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPBasicAuthConfig(cfg *managedserver.HTTPResourceServerBasicAuthConfig) *managedserver.HTTPResourceServerBasicAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	if normalized.Username == "" && normalized.Password == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPBearerConfig(cfg *managedserver.HTTPResourceServerBearerTokenConfig) *managedserver.HTTPResourceServerBearerTokenConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPCustomHeaderConfig(cfg *managedserver.HTTPResourceServerCustomHeaderConfig) *managedserver.HTTPResourceServerCustomHeaderConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Header = strings.TrimSpace(cfg.Header)
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Header == "" && normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPTLSConfig(cfg *managedserver.HTTPResourceServerTLSConfig) *managedserver.HTTPResourceServerTLSConfig {
	if cfg == nil {
		return nil
	}
	if !cfg.InsecureSkipVerify {
		return nil
	}
	normalized := *cfg
	return &normalized
}

func normalizeSecretsConfig(cfg *secrets.SecretsManagerConfig) *secrets.SecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.File = normalizeFileSecretsConfig(cfg.File)
	normalized.Vault = normalizeVaultSecretsConfig(cfg.Vault)
	if normalized.File == nil && normalized.Vault == nil {
		return nil
	}
	return &normalized
}

func normalizeFileSecretsConfig(cfg *secrets.FileSecretsManagerConfig) *secrets.FileSecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Path = strings.TrimSpace(cfg.Path)
	normalized.Key = strings.TrimSpace(cfg.Key)
	normalized.KeyFile = strings.TrimSpace(cfg.KeyFile)
	normalized.Passphrase = strings.TrimSpace(cfg.Passphrase)
	normalized.PassphraseFile = strings.TrimSpace(cfg.PassphraseFile)
	normalized.KDF = normalizeFileKDFConfig(cfg.KDF)
	if normalized.Path == "" && normalized.Key == "" && normalized.KeyFile == "" &&
		normalized.Passphrase == "" && normalized.PassphraseFile == "" && normalized.KDF == nil {
		return nil
	}
	return &normalized
}

func normalizeFileKDFConfig(cfg *secrets.FileSecretsManagerKDFConfig) *secrets.FileSecretsManagerKDFConfig {
	if cfg == nil {
		return nil
	}
	if cfg.Time == 0 && cfg.Memory == 0 && cfg.Threads == 0 {
		return nil
	}
	normalized := *cfg
	return &normalized
}

func normalizeVaultSecretsConfig(cfg *secrets.VaultSecretsManagerConfig) *secrets.VaultSecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Address = strings.TrimSpace(cfg.Address)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	normalized.PathPrefix = strings.TrimSpace(cfg.PathPrefix)
	normalized.Auth = normalizeVaultAuthConfig(cfg.Auth)
	normalized.TLS = normalizeVaultTLSConfig(cfg.TLS)
	if normalized.Address == "" && normalized.Mount == "" && normalized.PathPrefix == "" &&
		normalized.KVVersion == 0 && normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeVaultAuthConfig(cfg *secrets.VaultSecretsManagerAuthConfig) *secrets.VaultSecretsManagerAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	normalized.Password = normalizeVaultPasswordConfig(cfg.Password)
	normalized.AppRole = normalizeVaultAppRoleConfig(cfg.AppRole)
	if normalized.Token == "" && normalized.Password == nil && normalized.AppRole == nil {
		return nil
	}
	return &normalized
}

func normalizeVaultPasswordConfig(cfg *secrets.VaultSecretsManagerPasswordAuthConfig) *secrets.VaultSecretsManagerPasswordAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	if normalized.Username == "" && normalized.Password == "" && normalized.Mount == "" {
		return nil
	}
	return &normalized
}

func normalizeVaultAppRoleConfig(cfg *secrets.VaultSecretsManagerAppRoleAuthConfig) *secrets.VaultSecretsManagerAppRoleAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.RoleID = strings.TrimSpace(cfg.RoleID)
	normalized.SecretID = strings.TrimSpace(cfg.SecretID)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	if normalized.RoleID == "" && normalized.SecretID == "" && normalized.Mount == "" {
		return nil
	}
	return &normalized
}

func normalizeVaultTLSConfig(cfg *secrets.VaultSecretsManagerTLSConfig) *secrets.VaultSecretsManagerTLSConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.CACertFile = strings.TrimSpace(cfg.CACertFile)
	normalized.ClientCertFile = strings.TrimSpace(cfg.ClientCertFile)
	normalized.ClientKeyFile = strings.TrimSpace(cfg.ClientKeyFile)
	if normalized.CACertFile == "" && normalized.ClientCertFile == "" &&
		normalized.ClientKeyFile == "" && !normalized.InsecureSkipVerify {
		return nil
	}
	return &normalized
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
