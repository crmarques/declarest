package file

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
)

func validateCatalog(contextCatalog config.ContextCatalog) error {
	contextCatalog.DefaultEditor = strings.TrimSpace(contextCatalog.DefaultEditor)

	if len(contextCatalog.Contexts) == 0 {
		if contextCatalog.CurrentCtx != "" {
			return validationError("current-ctx must be empty when contexts list is empty", nil)
		}
		return nil
	}

	seen := map[string]struct{}{}
	for _, item := range contextCatalog.Contexts {
		if item.Name == "" {
			return validationError("context name must not be empty", nil)
		}
		if _, exists := seen[item.Name]; exists {
			return validationError(fmt.Sprintf("duplicate context name %q", item.Name), nil)
		}
		seen[item.Name] = struct{}{}

		if err := validateConfig(item); err != nil {
			return err
		}
	}

	if contextCatalog.CurrentCtx == "" {
		return validationError("current-ctx must be set when contexts are defined", nil)
	}

	if _, exists := seen[contextCatalog.CurrentCtx]; !exists {
		return validationError(fmt.Sprintf("current-ctx %q does not match any context", contextCatalog.CurrentCtx), nil)
	}

	return nil
}

func validateConfig(cfg config.Context) error {
	cfg = normalizeConfig(cfg)

	if cfg.Name == "" {
		return validationError("context name must not be empty", nil)
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
	if cfg.Metadata.BaseDir == "" {
		cfg.Metadata.BaseDir = contextRepositoryBaseDir(cfg)
	}
	return cfg
}

func compactConfigForPersistence(cfg config.Context) config.Context {
	if isDefaultMetadataBaseDir(cfg) {
		cfg.Metadata.BaseDir = ""
	}
	return cfg
}

func isDefaultMetadataBaseDir(cfg config.Context) bool {
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
		return validationError("repository.resource-format must be json or yaml", nil)
	}
	if repository.ResourceFormat == "" {
		repository.ResourceFormat = config.ResourceFormatJSON
	}
	if repository.Git == nil && repository.Filesystem == nil {
		return nil
	}

	if countSet(repository.Git != nil, repository.Filesystem != nil) != 1 {
		return validationError("repository must define exactly one of git or filesystem", nil)
	}

	if repository.Git != nil {
		if repository.Git.Local.BaseDir == "" {
			return validationError("repository.git.local.base-dir is required", nil)
		}
		if repository.Git.Remote != nil {
			if repository.Git.Remote.URL == "" {
				return validationError("repository.git.remote.url is required", nil)
			}
			if repository.Git.Remote.Auth != nil {
				if countSet(repository.Git.Remote.Auth.BasicAuth != nil, repository.Git.Remote.Auth.SSH != nil, repository.Git.Remote.Auth.AccessKey != nil) != 1 {
					return validationError("repository.git.remote.auth must define exactly one of basic-auth, ssh, access-key", nil)
				}
			}
		}
	}

	if repository.Filesystem != nil && repository.Filesystem.BaseDir == "" {
		return validationError("repository.filesystem.base-dir is required", nil)
	}

	return nil
}

func validateResourceServer(resourceServer *config.ResourceServer) error {
	if resourceServer == nil {
		return validationError("resource-server is required", nil)
	}
	if resourceServer.HTTP == nil {
		return validationError("resource-server must define http", nil)
	}
	if resourceServer.HTTP.BaseURL == "" {
		return validationError("resource-server.http.base-url is required", nil)
	}
	if resourceServer.HTTP.Auth == nil {
		return validationError("resource-server.http.auth is required", nil)
	}

	if countSet(
		resourceServer.HTTP.Auth.OAuth2 != nil,
		resourceServer.HTTP.Auth.BasicAuth != nil,
		resourceServer.HTTP.Auth.BearerToken != nil,
		resourceServer.HTTP.Auth.CustomHeader != nil,
	) != 1 {
		return validationError("resource-server.http.auth must define exactly one of oauth2, basic-auth, bearer-token, custom-header", nil)
	}

	if resourceServer.HTTP.Auth.OAuth2 != nil {
		oauth := resourceServer.HTTP.Auth.OAuth2
		if oauth.TokenURL == "" || oauth.GrantType == "" || oauth.ClientID == "" || oauth.ClientSecret == "" {
			return validationError("resource-server.http.auth.oauth2 requires token-url, grant-type, client-id, client-secret", nil)
		}
	}

	if resourceServer.HTTP.Auth.BasicAuth != nil {
		basic := resourceServer.HTTP.Auth.BasicAuth
		if basic.Username == "" || basic.Password == "" {
			return validationError("resource-server.http.auth.basic-auth requires username and password", nil)
		}
	}

	if resourceServer.HTTP.Auth.BearerToken != nil && resourceServer.HTTP.Auth.BearerToken.Token == "" {
		return validationError("resource-server.http.auth.bearer-token.token is required", nil)
	}

	if resourceServer.HTTP.Auth.CustomHeader != nil {
		head := resourceServer.HTTP.Auth.CustomHeader
		if head.Header == "" || head.Token == "" {
			return validationError("resource-server.http.auth.custom-header requires header and token", nil)
		}
	}

	return nil
}

func validateSecretStore(secretStore *config.SecretStore) error {
	if secretStore == nil {
		return nil
	}

	if countSet(secretStore.File != nil, secretStore.Vault != nil) != 1 {
		return validationError("secret-store must define exactly one of file or vault", nil)
	}

	if secretStore.File != nil {
		if secretStore.File.Path == "" {
			return validationError("secret-store.file.path is required", nil)
		}
		if countSet(
			secretStore.File.Key != "",
			secretStore.File.KeyFile != "",
			secretStore.File.Passphrase != "",
			secretStore.File.PassphraseFile != "",
		) != 1 {
			return validationError("secret-store.file must define exactly one of key, key-file, passphrase, passphrase-file", nil)
		}
	}

	if secretStore.Vault != nil {
		if secretStore.Vault.Address == "" {
			return validationError("secret-store.vault.address is required", nil)
		}
		if secretStore.Vault.Auth == nil {
			return validationError("secret-store.vault.auth is required", nil)
		}
		if countSet(
			secretStore.Vault.Auth.Token != "",
			secretStore.Vault.Auth.Password != nil,
			secretStore.Vault.Auth.AppRole != nil,
		) != 1 {
			return validationError("secret-store.vault.auth must define exactly one of token, password, approle", nil)
		}
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
				return config.Context{}, validationError("override repository.git.local.base-dir requires repository.git to be configured", nil)
			}
			cfg.Repository.Git.Local.BaseDir = value
		case "repository.filesystem.base-dir":
			if cfg.Repository.Filesystem == nil {
				return config.Context{}, validationError("override repository.filesystem.base-dir requires repository.filesystem to be configured", nil)
			}
			cfg.Repository.Filesystem.BaseDir = value
		case "resource-server.http.base-url":
			if cfg.ResourceServer == nil || cfg.ResourceServer.HTTP == nil {
				return config.Context{}, validationError("override resource-server.http.base-url requires resource-server.http to be configured", nil)
			}
			cfg.ResourceServer.HTTP.BaseURL = value
		case "metadata.base-dir":
			cfg.Metadata.BaseDir = value
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
