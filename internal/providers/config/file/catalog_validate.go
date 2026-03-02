package file

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func validateCatalog(contextCatalog config.ContextCatalog) error {
	contextCatalog.DefaultEditor = strings.TrimSpace(contextCatalog.DefaultEditor)

	if len(contextCatalog.Contexts) == 0 {
		if contextCatalog.CurrentCtx != "" {
			return faults.NewValidationError("current-ctx must be empty when contexts list is empty", nil)
		}
		return nil
	}

	seen := map[string]struct{}{}
	for _, item := range contextCatalog.Contexts {
		if item.Name == "" {
			return faults.NewValidationError("context name must not be empty", nil)
		}
		if _, exists := seen[item.Name]; exists {
			return faults.NewValidationError(fmt.Sprintf("duplicate context name %q", item.Name), nil)
		}
		seen[item.Name] = struct{}{}

		if err := validateConfig(item); err != nil {
			return err
		}
	}

	if contextCatalog.CurrentCtx == "" {
		return faults.NewValidationError("current-ctx must be set when contexts are defined", nil)
	}

	if _, exists := seen[contextCatalog.CurrentCtx]; !exists {
		return faults.NewValidationError(fmt.Sprintf("current-ctx %q does not match any context", contextCatalog.CurrentCtx), nil)
	}

	return nil
}

func validateConfig(cfg config.Context) error {
	cfg = normalizeConfig(cfg)

	if cfg.Name == "" {
		return faults.NewValidationError("context name must not be empty", nil)
	}

	if err := validateRepository(cfg.Repository); err != nil {
		return err
	}

	if err := validateResourceServer(cfg.ResourceServer); err != nil {
		return err
	}

	if err := validateSecretStore(cfg.SecretStore); err != nil {
		return err
	}
	if err := validateMetadata(cfg.Metadata); err != nil {
		return err
	}

	return nil
}

func normalizeConfig(cfg config.Context) config.Context {
	if cfg.Repository.ResourceFormat == "" {
		cfg.Repository.ResourceFormat = config.ResourceFormatJSON
	}
	return cfg
}

func applyConfigDefaults(cfg config.Context) config.Context {
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.Metadata.Bundle) == "" && cfg.Metadata.BaseDir == "" {
		cfg.Metadata.BaseDir = contextRepositoryBaseDir(cfg)
	}
	return cfg
}

func compactConfigForPersistence(cfg config.Context) config.Context {
	if strings.TrimSpace(cfg.Metadata.Bundle) != "" {
		cfg.Metadata.BaseDir = ""
		return cfg
	}
	if isDefaultMetadataBaseDir(cfg) {
		cfg.Metadata.BaseDir = ""
	}
	return cfg
}

func isDefaultMetadataBaseDir(cfg config.Context) bool {
	if strings.TrimSpace(cfg.Metadata.Bundle) != "" {
		return false
	}
	repoBaseDir := normalizeBaseDirPath(contextRepositoryBaseDir(cfg))
	metadataBaseDir := normalizeBaseDirPath(cfg.Metadata.BaseDir)
	return repoBaseDir != "" && metadataBaseDir != "" && repoBaseDir == metadataBaseDir
}

func contextRepositoryBaseDir(cfg config.Context) string {
	switch {
	case cfg.Repository.Git != nil:
		return cfg.Repository.Git.Local.BaseDir
	case cfg.Repository.Filesystem != nil:
		return cfg.Repository.Filesystem.BaseDir
	default:
		return ""
	}
}

func normalizeBaseDirPath(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func validateRepository(repository config.Repository) error {
	if repository.ResourceFormat != "" &&
		repository.ResourceFormat != config.ResourceFormatJSON &&
		repository.ResourceFormat != config.ResourceFormatYAML {
		return faults.NewValidationError("repository.resource-format must be json or yaml", nil)
	}
	if repository.ResourceFormat == "" {
		repository.ResourceFormat = config.ResourceFormatJSON
	}
	if repository.Git == nil && repository.Filesystem == nil {
		return nil
	}

	if countSet(repository.Git != nil, repository.Filesystem != nil) != 1 {
		return faults.NewValidationError("repository must define exactly one of git or filesystem", nil)
	}

	if repository.Git != nil {
		if repository.Git.Local.BaseDir == "" {
			return faults.NewValidationError("repository.git.local.base-dir is required", nil)
		}
		if repository.Git.Remote != nil {
			if repository.Git.Remote.URL == "" {
				return faults.NewValidationError("repository.git.remote.url is required", nil)
			}
			if repository.Git.Remote.Auth != nil {
				if countSet(repository.Git.Remote.Auth.BasicAuth != nil, repository.Git.Remote.Auth.SSH != nil, repository.Git.Remote.Auth.AccessKey != nil) != 1 {
					return faults.NewValidationError("repository.git.remote.auth must define exactly one of basic-auth, ssh, access-key", nil)
				}
			}
		}
	}

	if repository.Filesystem != nil && repository.Filesystem.BaseDir == "" {
		return faults.NewValidationError("repository.filesystem.base-dir is required", nil)
	}

	return nil
}

func validateResourceServer(resourceServer *config.ResourceServer) error {
	if resourceServer == nil {
		return faults.NewValidationError("managed-server is required", nil)
	}
	if resourceServer.HTTP == nil {
		return faults.NewValidationError("managed-server must define http", nil)
	}
	if resourceServer.HTTP.BaseURL == "" {
		return faults.NewValidationError("managed-server.http.base-url is required", nil)
	}
	if resourceServer.HTTP.Auth == nil {
		return faults.NewValidationError("managed-server.http.auth is required", nil)
	}

	if countSet(
		resourceServer.HTTP.Auth.OAuth2 != nil,
		resourceServer.HTTP.Auth.BasicAuth != nil,
		len(resourceServer.HTTP.Auth.CustomHeaders) > 0,
	) != 1 {
		return faults.NewValidationError("managed-server.http.auth must define exactly one of oauth2, basic-auth, custom-headers", nil)
	}

	if resourceServer.HTTP.Auth.OAuth2 != nil {
		oauth := resourceServer.HTTP.Auth.OAuth2
		if oauth.TokenURL == "" || oauth.GrantType == "" || oauth.ClientID == "" || oauth.ClientSecret == "" {
			return faults.NewValidationError("managed-server.http.auth.oauth2 requires token-url, grant-type, client-id, client-secret", nil)
		}
	}

	if resourceServer.HTTP.Auth.BasicAuth != nil {
		basic := resourceServer.HTTP.Auth.BasicAuth
		if basic.Username == "" || basic.Password == "" {
			return faults.NewValidationError("managed-server.http.auth.basic-auth requires username and password", nil)
		}
	}

	for idx, head := range resourceServer.HTTP.Auth.CustomHeaders {
		if head.Header == "" || head.Value == "" {
			return faults.NewValidationError(
				fmt.Sprintf("managed-server.http.auth.custom-headers[%d] requires header and value", idx),
				nil,
			)
		}
	}

	if err := validateResourceServerProxy(resourceServer.HTTP.Proxy); err != nil {
		return err
	}

	return nil
}

func validateResourceServerProxy(proxy *config.HTTPProxy) error {
	if proxy == nil {
		return nil
	}

	httpURL := strings.TrimSpace(proxy.HTTPURL)
	httpsURL := strings.TrimSpace(proxy.HTTPSURL)

	if httpURL == "" && httpsURL == "" {
		return faults.NewValidationError("managed-server.http.proxy must define at least one of http-url or https-url", nil)
	}

	if httpURL != "" {
		if err := validateResourceServerProxyURL("managed-server.http.proxy.http-url", httpURL); err != nil {
			return err
		}
	}

	if httpsURL != "" {
		if err := validateResourceServerProxyURL("managed-server.http.proxy.https-url", httpsURL); err != nil {
			return err
		}
	}

	if proxy.Auth != nil {
		if strings.TrimSpace(proxy.Auth.Username) == "" || strings.TrimSpace(proxy.Auth.Password) == "" {
			return faults.NewValidationError("managed-server.http.proxy.auth requires username and password", nil)
		}
	}

	return nil
}

func validateResourceServerProxyURL(field string, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return faults.NewValidationError(field+" is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return faults.NewValidationError(field+" must use http or https", nil)
	}
	if parsed.Host == "" {
		return faults.NewValidationError(field+" host is required", nil)
	}
	return nil
}

func validateSecretStore(secretStore *config.SecretStore) error {
	if secretStore == nil {
		return nil
	}

	if countSet(secretStore.File != nil, secretStore.Vault != nil) != 1 {
		return faults.NewValidationError("secret-store must define exactly one of file or vault", nil)
	}

	if secretStore.File != nil {
		if secretStore.File.Path == "" {
			return faults.NewValidationError("secret-store.file.path is required", nil)
		}
		if countSet(
			secretStore.File.Key != "",
			secretStore.File.KeyFile != "",
			secretStore.File.Passphrase != "",
			secretStore.File.PassphraseFile != "",
		) != 1 {
			return faults.NewValidationError("secret-store.file must define exactly one of key, key-file, passphrase, passphrase-file", nil)
		}
	}

	if secretStore.Vault != nil {
		if secretStore.Vault.Address == "" {
			return faults.NewValidationError("secret-store.vault.address is required", nil)
		}
		if secretStore.Vault.Auth == nil {
			return faults.NewValidationError("secret-store.vault.auth is required", nil)
		}
		if countSet(
			secretStore.Vault.Auth.Token != "",
			secretStore.Vault.Auth.Password != nil,
			secretStore.Vault.Auth.AppRole != nil,
		) != 1 {
			return faults.NewValidationError("secret-store.vault.auth must define exactly one of token, password, approle", nil)
		}
	}

	return nil
}

func validateMetadata(metadata config.Metadata) error {
	baseDir := strings.TrimSpace(metadata.BaseDir)
	bundle := strings.TrimSpace(metadata.Bundle)

	if baseDir != "" && bundle != "" {
		return faults.NewValidationError("metadata must define at most one of base-dir or bundle", nil)
	}

	return nil
}

func applyOverrides(cfg config.Context, overrides map[string]string) (config.Context, error) {
	for _, key := range sortedOverrideKeys(overrides) {
		value := overrides[key]
		switch key {
		case "repository.resource-format":
			cfg.Repository.ResourceFormat = value
		case "repository.git.local.base-dir":
			if cfg.Repository.Git == nil {
				return config.Context{}, faults.NewValidationError("override repository.git.local.base-dir requires repository.git to be configured", nil)
			}
			cfg.Repository.Git.Local.BaseDir = value
		case "repository.filesystem.base-dir":
			if cfg.Repository.Filesystem == nil {
				return config.Context{}, faults.NewValidationError("override repository.filesystem.base-dir requires repository.filesystem to be configured", nil)
			}
			cfg.Repository.Filesystem.BaseDir = value
		case "managed-server.http.base-url":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.base-url requires managed-server.http to be configured", nil)
			}
			cfg.ResourceServer.HTTP.BaseURL = value
		case "managed-server.http.proxy.http-url":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.http-url requires managed-server.http to be configured", nil)
			}
			if cfg.ResourceServer.HTTP.Proxy == nil {
				cfg.ResourceServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ResourceServer.HTTP.Proxy.HTTPURL = value
		case "managed-server.http.proxy.https-url":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.https-url requires managed-server.http to be configured", nil)
			}
			if cfg.ResourceServer.HTTP.Proxy == nil {
				cfg.ResourceServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ResourceServer.HTTP.Proxy.HTTPSURL = value
		case "managed-server.http.proxy.no-proxy":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.no-proxy requires managed-server.http to be configured", nil)
			}
			if cfg.ResourceServer.HTTP.Proxy == nil {
				cfg.ResourceServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ResourceServer.HTTP.Proxy.NoProxy = value
		case "managed-server.http.proxy.auth.username":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.auth.username requires managed-server.http to be configured", nil)
			}
			if cfg.ResourceServer.HTTP.Proxy == nil {
				cfg.ResourceServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			if cfg.ResourceServer.HTTP.Proxy.Auth == nil {
				cfg.ResourceServer.HTTP.Proxy.Auth = &config.ProxyAuth{}
			}
			cfg.ResourceServer.HTTP.Proxy.Auth.Username = value
		case "managed-server.http.proxy.auth.password":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.auth.password requires managed-server.http to be configured", nil)
			}
			if cfg.ResourceServer.HTTP.Proxy == nil {
				cfg.ResourceServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			if cfg.ResourceServer.HTTP.Proxy.Auth == nil {
				cfg.ResourceServer.HTTP.Proxy.Auth = &config.ProxyAuth{}
			}
			cfg.ResourceServer.HTTP.Proxy.Auth.Password = value
		case "metadata.base-dir":
			cfg.Metadata.BaseDir = value
			if strings.TrimSpace(value) != "" {
				cfg.Metadata.Bundle = ""
			}
		case "metadata.bundle":
			cfg.Metadata.Bundle = value
			if strings.TrimSpace(value) != "" {
				cfg.Metadata.BaseDir = ""
			}
		default:
			return config.Context{}, unknownOverrideError(key)
		}
	}

	return cfg, nil
}

func sortedOverrideKeys(overrides map[string]string) []string {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
