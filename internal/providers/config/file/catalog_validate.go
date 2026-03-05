package file

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
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

	if err := validateManagedServer(cfg.ManagedServer); err != nil {
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
	if cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil {
		cfg.Repository.Git.Remote.Proxy = normalizeProxy(cfg.Repository.Git.Remote.Proxy)
	}
	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		cfg.ManagedServer.HTTP.HealthCheck = strings.TrimSpace(cfg.ManagedServer.HTTP.HealthCheck)
		cfg.ManagedServer.HTTP.Proxy = normalizeProxy(cfg.ManagedServer.HTTP.Proxy)
	}
	if cfg.SecretStore != nil && cfg.SecretStore.Vault != nil {
		cfg.SecretStore.Vault.Proxy = normalizeProxy(cfg.SecretStore.Vault.Proxy)
	}
	cfg.Metadata.Proxy = normalizeProxy(cfg.Metadata.Proxy)
	return cfg
}

func applyConfigDefaults(cfg config.Context) config.Context {
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.Metadata.Bundle) == "" && strings.TrimSpace(cfg.Metadata.BundleFile) == "" && cfg.Metadata.BaseDir == "" {
		cfg.Metadata.BaseDir = contextRepositoryBaseDir(cfg)
	}
	cfg = applyProxyDefaults(cfg)
	return cfg
}

func applyProxyDefaults(cfg config.Context) config.Context {
	targets := buildProxyTargets(&cfg)
	var canonical *config.HTTPProxy
	for _, target := range targets {
		current := *target.proxy
		if current != nil && proxyhelper.HasURLs(current) {
			canonical = proxyhelper.Clone(current)
			break
		}
	}
	if canonical == nil {
		return cfg
	}

	for _, target := range targets {
		current := *target.proxy
		if current == nil {
			*target.proxy = proxyhelper.Clone(canonical)
			continue
		}
		if proxyhelper.IsExplicitDisable(current) {
			continue
		}
	}

	return cfg
}

func preserveProxyOmissions(cfg config.Context, baseline config.Context) config.Context {
	targets := buildProxyTargets(&cfg)
	baselineTargets := buildProxyTargets(&baseline)
	if len(targets) == 0 || len(baselineTargets) == 0 {
		return cfg
	}

	baselineByName := make(map[string]*config.HTTPProxy, len(baselineTargets))
	for _, target := range baselineTargets {
		baselineByName[target.name] = *target.proxy
	}

	var canonical *config.HTTPProxy
	for _, target := range targets {
		current := *target.proxy
		if current != nil && proxyhelper.HasURLs(current) {
			canonical = current
			break
		}
	}
	if canonical == nil {
		return cfg
	}

	for _, target := range targets {
		current := *target.proxy
		if current == nil || proxyhelper.IsExplicitDisable(current) {
			continue
		}
		existing, ok := baselineByName[target.name]
		if !ok || existing != nil {
			continue
		}
		if proxyhelper.Equal(current, canonical) {
			*target.proxy = nil
		}
	}

	return cfg
}

type proxyTarget struct {
	name  string
	proxy **config.HTTPProxy
}

func buildProxyTargets(cfg *config.Context) []proxyTarget {
	targets := make([]proxyTarget, 0, 4)
	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		targets = append(targets, proxyTarget{
			name:  "managed-server.http.proxy",
			proxy: &cfg.ManagedServer.HTTP.Proxy,
		})
	}
	if cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil {
		targets = append(targets, proxyTarget{
			name:  "repository.git.remote.proxy",
			proxy: &cfg.Repository.Git.Remote.Proxy,
		})
	}
	if cfg.SecretStore != nil && cfg.SecretStore.Vault != nil {
		targets = append(targets, proxyTarget{
			name:  "secret-store.vault.proxy",
			proxy: &cfg.SecretStore.Vault.Proxy,
		})
	}
	targets = append(targets, proxyTarget{
		name:  "metadata.proxy",
		proxy: &cfg.Metadata.Proxy,
	})
	return targets
}

func normalizeProxy(proxy *config.HTTPProxy) *config.HTTPProxy {
	if proxy == nil {
		return nil
	}

	normalized := &config.HTTPProxy{
		HTTPURL:  strings.TrimSpace(proxy.HTTPURL),
		HTTPSURL: strings.TrimSpace(proxy.HTTPSURL),
		NoProxy:  strings.TrimSpace(proxy.NoProxy),
	}
	if proxy.Auth != nil {
		normalized.Auth = &config.ProxyAuth{
			Username: strings.TrimSpace(proxy.Auth.Username),
			Password: strings.TrimSpace(proxy.Auth.Password),
		}
	}
	return normalized
}

func compactConfigForPersistence(cfg config.Context) config.Context {
	return config.CompactContext(cfg)
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
			if err := validateProxy("repository.git.remote.proxy", repository.Git.Remote.Proxy); err != nil {
				return err
			}
		}
	}

	if repository.Filesystem != nil && repository.Filesystem.BaseDir == "" {
		return faults.NewValidationError("repository.filesystem.base-dir is required", nil)
	}

	return nil
}

func validateManagedServer(resourceServer *config.ManagedServer) error {
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

	if err := validateManagedServerProxy(resourceServer.HTTP.Proxy); err != nil {
		return err
	}
	if err := validateManagedServerRequestThrottling(resourceServer.HTTP.RequestThrottling); err != nil {
		return err
	}
	if err := validateManagedServerHealthCheck(resourceServer.HTTP.HealthCheck); err != nil {
		return err
	}

	return nil
}

func validateManagedServerProxy(proxy *config.HTTPProxy) error {
	return validateProxy("managed-server.http.proxy", proxy)
}

func validateManagedServerRequestThrottling(throttling *config.HTTPRequestThrottling) error {
	if throttling == nil {
		return nil
	}
	if throttling.MaxConcurrentRequests <= 0 && throttling.RequestsPerSecond <= 0 {
		return faults.NewValidationError("managed-server.http.request-throttling must define at least one of max-concurrent-requests or requests-per-second", nil)
	}
	if throttling.MaxConcurrentRequests < 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.max-concurrent-requests must be greater than zero when set", nil)
	}
	if throttling.QueueSize < 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.queue-size must be greater than or equal to zero", nil)
	}
	if throttling.QueueSize > 0 && throttling.MaxConcurrentRequests <= 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.queue-size requires max-concurrent-requests", nil)
	}
	if throttling.RequestsPerSecond < 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.requests-per-second must be greater than zero when set", nil)
	}
	if throttling.Burst < 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.burst must be greater than zero when set", nil)
	}
	if throttling.Burst > 0 && throttling.RequestsPerSecond <= 0 {
		return faults.NewValidationError("managed-server.http.request-throttling.burst requires requests-per-second", nil)
	}
	return nil
}

func validateManagedServerHealthCheck(value string) error {
	healthCheck := strings.TrimSpace(value)
	if healthCheck == "" {
		return nil
	}

	parsed, err := url.Parse(healthCheck)
	if err != nil {
		return faults.NewValidationError("managed-server.http.health-check is invalid", err)
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return faults.NewValidationError("managed-server.http.health-check must not include query parameters", nil)
	}

	// Relative paths are interpreted against managed-server.http.base-url.
	if parsed.Scheme == "" && parsed.Host == "" {
		if strings.TrimSpace(parsed.Path) == "" {
			return faults.NewValidationError("managed-server.http.health-check is invalid", nil)
		}
		return nil
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return faults.NewValidationError("managed-server.http.health-check URL must use http or https", nil)
	}
	if parsed.Host == "" {
		return faults.NewValidationError("managed-server.http.health-check URL host is required", nil)
	}

	_, err = filepath.Rel("/", parsed.Path)
	if err != nil {
		return faults.NewValidationError("managed-server.http.health-check URL path is invalid", err)
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
		if err := validateProxy("secret-store.vault.proxy", secretStore.Vault.Proxy); err != nil {
			return err
		}
	}

	return nil
}

func validateMetadata(metadata config.Metadata) error {
	baseDir := strings.TrimSpace(metadata.BaseDir)
	bundle := strings.TrimSpace(metadata.Bundle)
	bundleFile := strings.TrimSpace(metadata.BundleFile)

	if countSet(baseDir != "", bundle != "", bundleFile != "") > 1 {
		return faults.NewValidationError("metadata must define at most one of base-dir, bundle, or bundle-file", nil)
	}
	if err := validateProxy("metadata.proxy", metadata.Proxy); err != nil {
		return err
	}

	return nil
}

func validateProxy(field string, proxy *config.HTTPProxy) error {
	if proxy == nil || proxyhelper.IsExplicitDisable(proxy) {
		return nil
	}
	if !proxyhelper.HasURLs(proxy) {
		return faults.NewValidationError(field+" must define at least one of http-url or https-url", nil)
	}
	if _, err := proxyhelper.Build(field, proxy); err != nil {
		return err
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
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.base-url requires managed-server.http to be configured", nil)
			}
			cfg.ManagedServer.HTTP.BaseURL = value
		case "managed-server.http.health-check":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.health-check requires managed-server.http to be configured", nil)
			}
			cfg.ManagedServer.HTTP.HealthCheck = value
		case "managed-server.http.proxy.http-url":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.http-url requires managed-server.http to be configured", nil)
			}
			if cfg.ManagedServer.HTTP.Proxy == nil {
				cfg.ManagedServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedServer.HTTP.Proxy.HTTPURL = value
		case "managed-server.http.proxy.https-url":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.https-url requires managed-server.http to be configured", nil)
			}
			if cfg.ManagedServer.HTTP.Proxy == nil {
				cfg.ManagedServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedServer.HTTP.Proxy.HTTPSURL = value
		case "managed-server.http.proxy.no-proxy":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.no-proxy requires managed-server.http to be configured", nil)
			}
			if cfg.ManagedServer.HTTP.Proxy == nil {
				cfg.ManagedServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedServer.HTTP.Proxy.NoProxy = value
		case "managed-server.http.proxy.auth.username":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.auth.username requires managed-server.http to be configured", nil)
			}
			if cfg.ManagedServer.HTTP.Proxy == nil {
				cfg.ManagedServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			if cfg.ManagedServer.HTTP.Proxy.Auth == nil {
				cfg.ManagedServer.HTTP.Proxy.Auth = &config.ProxyAuth{}
			}
			cfg.ManagedServer.HTTP.Proxy.Auth.Username = value
		case "managed-server.http.proxy.auth.password":
			if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managed-server.http.proxy.auth.password requires managed-server.http to be configured", nil)
			}
			if cfg.ManagedServer.HTTP.Proxy == nil {
				cfg.ManagedServer.HTTP.Proxy = &config.HTTPProxy{}
			}
			if cfg.ManagedServer.HTTP.Proxy.Auth == nil {
				cfg.ManagedServer.HTTP.Proxy.Auth = &config.ProxyAuth{}
			}
			cfg.ManagedServer.HTTP.Proxy.Auth.Password = value
		case "metadata.base-dir":
			cfg.Metadata.BaseDir = value
			if strings.TrimSpace(value) != "" {
				cfg.Metadata.Bundle = ""
				cfg.Metadata.BundleFile = ""
			}
		case "metadata.bundle":
			cfg.Metadata.Bundle = value
			if strings.TrimSpace(value) != "" {
				cfg.Metadata.BaseDir = ""
				cfg.Metadata.BundleFile = ""
			}
		case "metadata.bundle-file":
			cfg.Metadata.BundleFile = value
			if strings.TrimSpace(value) != "" {
				cfg.Metadata.BaseDir = ""
				cfg.Metadata.Bundle = ""
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
