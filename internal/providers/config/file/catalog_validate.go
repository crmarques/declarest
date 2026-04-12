// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package file

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/envref"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

func validateCatalog(contextCatalog config.ContextCatalog) error {
	contextCatalog.DefaultEditor = strings.TrimSpace(contextCatalog.DefaultEditor)
	credentials, err := validateCredentials(contextCatalog.Credentials)
	if err != nil {
		return err
	}

	if len(contextCatalog.Contexts) == 0 {
		if contextCatalog.CurrentContext != "" {
			return faults.NewValidationError("currentContext must be empty when contexts list is empty", nil)
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

		if err := validateConfig(item, credentials, true); err != nil {
			return err
		}
	}

	if contextCatalog.CurrentContext == "" {
		return faults.NewValidationError("currentContext must be set when contexts are defined", nil)
	}

	if _, exists := seen[contextCatalog.CurrentContext]; !exists {
		return faults.NewValidationError(fmt.Sprintf("currentContext %q does not match any context", contextCatalog.CurrentContext), nil)
	}

	return nil
}

func validateResolvedCatalog(contextCatalog config.ContextCatalog) error {
	return validateCatalog(envref.ExpandExactEnvPlaceholders(contextCatalog))
}

func validateConfig(cfg config.Context, credentials map[string]config.Credential, strictCredentialRefs bool) error {
	cfg = normalizeConfig(cfg)

	if cfg.Name == "" {
		return faults.NewValidationError("context name must not be empty", nil)
	}
	if cfg.Repository.Git == nil && cfg.Repository.Filesystem == nil && cfg.ManagedService == nil {
		return faults.NewValidationError("context must define at least one of repository or managedService", nil)
	}

	if err := validateRepository(cfg.Repository, credentials, strictCredentialRefs); err != nil {
		return err
	}

	if err := validateManagedService(cfg.ManagedService, credentials, strictCredentialRefs); err != nil {
		return err
	}

	if err := validateSecretStore(cfg.SecretStore, credentials, strictCredentialRefs); err != nil {
		return err
	}
	if err := validateMetadata(cfg.Metadata); err != nil {
		return err
	}

	return nil
}

func validateResolvedConfig(cfg config.Context) error {
	return validateConfig(envref.ExpandExactEnvPlaceholders(cfg), cfg.Credentials, false)
}

func normalizeConfig(cfg config.Context) config.Context {
	if cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil {
		if cfg.Repository.Git.Remote.Auth != nil && cfg.Repository.Git.Remote.Auth.Basic != nil {
			cfg.Repository.Git.Remote.Auth.Basic.CredentialsRef = normalizeCredentialsRef(cfg.Repository.Git.Remote.Auth.Basic.CredentialsRef)
		}
		cfg.Repository.Git.Remote.Proxy = normalizeProxy(cfg.Repository.Git.Remote.Proxy)
	}
	if cfg.ManagedService != nil && cfg.ManagedService.HTTP != nil {
		cfg.ManagedService.HTTP.HealthCheck = strings.TrimSpace(cfg.ManagedService.HTTP.HealthCheck)
		if cfg.ManagedService.HTTP.Auth != nil && cfg.ManagedService.HTTP.Auth.Basic != nil {
			cfg.ManagedService.HTTP.Auth.Basic.CredentialsRef = normalizeCredentialsRef(cfg.ManagedService.HTTP.Auth.Basic.CredentialsRef)
		}
		cfg.ManagedService.HTTP.Proxy = normalizeProxy(cfg.ManagedService.HTTP.Proxy)
	}
	if cfg.SecretStore != nil && cfg.SecretStore.Vault != nil {
		if cfg.SecretStore.Vault.Auth != nil && cfg.SecretStore.Vault.Auth.Password != nil {
			cfg.SecretStore.Vault.Auth.Password.CredentialsRef = normalizeCredentialsRef(cfg.SecretStore.Vault.Auth.Password.CredentialsRef)
		}
		cfg.SecretStore.Vault.Proxy = normalizeProxy(cfg.SecretStore.Vault.Proxy)
	}
	cfg.Metadata.Proxy = normalizeProxy(cfg.Metadata.Proxy)
	return cfg
}

func applyConfigDefaults(cfg config.Context) config.Context {
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.Metadata.Bundle) == "" && strings.TrimSpace(cfg.Metadata.BundleFile) == "" && cfg.Metadata.BaseDir == "" {
		cfg.Metadata.BaseDir = config.ContextRepositoryBaseDir(cfg)
	}
	cfg = applyProxyDefaults(cfg)
	return cfg
}

func applyProxyDefaults(cfg config.Context) config.Context {
	envProxy := proxyhelper.FromEnvironment()
	targets := buildProxyTargets(&cfg)
	var canonical *config.HTTPProxy
	if envProxy != nil && proxyhelper.HasURLs(envProxy) {
		canonical = proxyhelper.Clone(envProxy)
	}
	canonicalFromConfig := false

	for _, target := range targets {
		current := *target.proxy
		if current == nil || proxyhelper.IsExplicitDisable(current) {
			continue
		}

		merged := proxyhelper.Merge(envProxy, current)
		*target.proxy = merged
		if !canonicalFromConfig && proxyhelper.HasURLs(merged) {
			canonical = proxyhelper.Clone(merged)
			canonicalFromConfig = true
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
	if cfg.ManagedService != nil && cfg.ManagedService.HTTP != nil {
		targets = append(targets, proxyTarget{
			name:  "managedService.http.proxy",
			proxy: &cfg.ManagedService.HTTP.Proxy,
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
			name:  "secretStore.vault.proxy",
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
		normalized.Auth = &config.ProxyAuth{}
		if proxy.Auth.Basic != nil {
			normalized.Auth.Basic = &config.BasicAuth{
				CredentialsRef: normalizeCredentialsRef(proxy.Auth.Basic.CredentialsRef),
			}
		}
	}
	return normalized
}

func normalizeCredentialsRef(ref *config.CredentialsRef) *config.CredentialsRef {
	if ref == nil {
		return nil
	}
	normalized := *ref
	normalized.Name = strings.TrimSpace(normalized.Name)
	return &normalized
}

func compactConfigForPersistence(cfg config.Context) config.Context {
	return config.CompactContext(cfg)
}

func validateRepository(
	repository config.Repository,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if repository.Git == nil && repository.Filesystem == nil {
		return nil
	}

	if countSet(repository.Git != nil, repository.Filesystem != nil) != 1 {
		return faults.NewValidationError("repository must define exactly one of git or filesystem", nil)
	}

	if repository.Git != nil {
		if repository.Git.Local.BaseDir == "" {
			return faults.NewValidationError("repository.git.local.baseDir is required", nil)
		}
		if repository.Git.Remote != nil {
			if repository.Git.Remote.URL == "" {
				return faults.NewValidationError("repository.git.remote.url is required", nil)
			}
			if repository.Git.Remote.Auth != nil {
				if countSet(
					repository.Git.Remote.Auth.Basic != nil,
					repository.Git.Remote.Auth.SSH != nil,
					repository.Git.Remote.Auth.AccessKey != nil,
				) != 1 {
					return faults.NewValidationError("repository.git.remote.auth must define exactly one of basic, ssh, accessKey", nil)
				}
				if repository.Git.Remote.Auth.Basic != nil {
					if err := validateCredentialRef(
						"repository.git.remote.auth.basic.credentialsRef",
						repository.Git.Remote.Auth.Basic.CredentialsRef,
						credentials,
						strictCredentialRefs,
					); err != nil {
						return err
					}
				}
			}
			if err := validateProxy("repository.git.remote.proxy", repository.Git.Remote.Proxy, credentials, strictCredentialRefs); err != nil {
				return err
			}
		}
	}

	if repository.Filesystem != nil && repository.Filesystem.BaseDir == "" {
		return faults.NewValidationError("repository.filesystem.baseDir is required", nil)
	}

	return nil
}

func validateManagedService(
	resourceServer *config.ManagedService,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if resourceServer == nil {
		return nil
	}
	if resourceServer.HTTP == nil {
		return faults.NewValidationError("managedService must define http", nil)
	}
	if resourceServer.HTTP.BaseURL == "" {
		return faults.NewValidationError("managedService.http.url is required", nil)
	}
	if resourceServer.HTTP.Auth == nil {
		return faults.NewValidationError("managedService.http.auth is required", nil)
	}

	if countSet(
		resourceServer.HTTP.Auth.OAuth2 != nil,
		resourceServer.HTTP.Auth.Basic != nil,
		len(resourceServer.HTTP.Auth.CustomHeaders) > 0,
	) != 1 {
		return faults.NewValidationError("managedService.http.auth must define exactly one of oauth2, basic, customHeaders", nil)
	}

	if resourceServer.HTTP.Auth.OAuth2 != nil {
		oauth := resourceServer.HTTP.Auth.OAuth2
		if oauth.TokenURL == "" || oauth.GrantType == "" || oauth.ClientID == "" || oauth.ClientSecret == "" {
			return faults.NewValidationError("managedService.http.auth.oauth2 requires tokenURL, grantType, clientID, clientSecret", nil)
		}
	}

	if resourceServer.HTTP.Auth.Basic != nil {
		basic := resourceServer.HTTP.Auth.Basic
		if err := validateCredentialRef(
			"managedService.http.auth.basic.credentialsRef",
			basic.CredentialsRef,
			credentials,
			strictCredentialRefs,
		); err != nil {
			return err
		}
	}

	for idx, head := range resourceServer.HTTP.Auth.CustomHeaders {
		if head.Header == "" || head.Value == "" {
			return faults.NewValidationError(
				fmt.Sprintf("managedService.http.auth.customHeaders[%d] requires header and value", idx),
				nil,
			)
		}
	}

	if err := validateManagedServiceProxy(resourceServer.HTTP.Proxy, credentials, strictCredentialRefs); err != nil {
		return err
	}
	if err := validateManagedServiceRequestThrottling(resourceServer.HTTP.RequestThrottling); err != nil {
		return err
	}
	if err := validateManagedServiceHealthCheck(resourceServer.HTTP.HealthCheck); err != nil {
		return err
	}

	return nil
}

func validateManagedServiceProxy(
	proxy *config.HTTPProxy,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	return validateProxy("managedService.http.proxy", proxy, credentials, strictCredentialRefs)
}

func validateManagedServiceRequestThrottling(throttling *config.HTTPRequestThrottling) error {
	if throttling == nil {
		return nil
	}
	if throttling.MaxConcurrentRequests <= 0 && throttling.RequestsPerSecond <= 0 {
		return faults.NewValidationError("managedService.http.requestThrottling must define at least one of maxConcurrentRequests or requestsPerSecond", nil)
	}
	if throttling.MaxConcurrentRequests < 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.maxConcurrentRequests must be greater than zero when set", nil)
	}
	if throttling.QueueSize < 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.queueSize must be greater than or equal to zero", nil)
	}
	if throttling.QueueSize > 0 && throttling.MaxConcurrentRequests <= 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.queueSize requires maxConcurrentRequests", nil)
	}
	if throttling.RequestsPerSecond < 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.requestsPerSecond must be greater than zero when set", nil)
	}
	if throttling.Burst < 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.burst must be greater than zero when set", nil)
	}
	if throttling.Burst > 0 && throttling.RequestsPerSecond <= 0 {
		return faults.NewValidationError("managedService.http.requestThrottling.burst requires requestsPerSecond", nil)
	}
	return nil
}

func validateManagedServiceHealthCheck(value string) error {
	healthCheck := strings.TrimSpace(value)
	if healthCheck == "" {
		return nil
	}

	parsed, err := url.Parse(healthCheck)
	if err != nil {
		return faults.NewValidationError("managedService.http.healthCheck is invalid", err)
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return faults.NewValidationError("managedService.http.healthCheck must not include query parameters", nil)
	}

	// Relative paths are interpreted against managedService.http.url.
	if parsed.Scheme == "" && parsed.Host == "" {
		if strings.TrimSpace(parsed.Path) == "" {
			return faults.NewValidationError("managedService.http.healthCheck is invalid", nil)
		}
		return nil
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return faults.NewValidationError("managedService.http.healthCheck URL must use http or https", nil)
	}
	if parsed.Host == "" {
		return faults.NewValidationError("managedService.http.healthCheck URL host is required", nil)
	}

	_, err = filepath.Rel("/", parsed.Path)
	if err != nil {
		return faults.NewValidationError("managedService.http.healthCheck URL path is invalid", err)
	}

	return nil
}

func validateSecretStore(
	secretStore *config.SecretStore,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if secretStore == nil {
		return nil
	}

	if countSet(secretStore.File != nil, secretStore.Vault != nil) != 1 {
		return faults.NewValidationError("secretStore must define exactly one of file or vault", nil)
	}

	if secretStore.File != nil {
		if secretStore.File.Path == "" {
			return faults.NewValidationError("secretStore.file.path is required", nil)
		}
		if countSet(
			secretStore.File.Key != "",
			secretStore.File.KeyFile != "",
			secretStore.File.Passphrase != "",
			secretStore.File.PassphraseFile != "",
		) != 1 {
			return faults.NewValidationError("secretStore.file must define exactly one of key, keyFile, passphrase, passphraseFile", nil)
		}
	}

	if secretStore.Vault != nil {
		if secretStore.Vault.Address == "" {
			return faults.NewValidationError("secretStore.vault.address is required", nil)
		}
		if secretStore.Vault.Auth == nil {
			return faults.NewValidationError("secretStore.vault.auth is required", nil)
		}
		if countSet(
			secretStore.Vault.Auth.Token != "",
			secretStore.Vault.Auth.Password != nil,
			secretStore.Vault.Auth.AppRole != nil,
		) != 1 {
			return faults.NewValidationError("secretStore.vault.auth must define exactly one of token, password, appRole", nil)
		}
		if secretStore.Vault.Auth.Password != nil {
			if err := validateCredentialRef(
				"secretStore.vault.auth.password.credentialsRef",
				secretStore.Vault.Auth.Password.CredentialsRef,
				credentials,
				strictCredentialRefs,
			); err != nil {
				return err
			}
		}
		if err := validateProxy("secretStore.vault.proxy", secretStore.Vault.Proxy, credentials, strictCredentialRefs); err != nil {
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
		return faults.NewValidationError("metadata must define at most one of baseDir, bundle, or bundleFile", nil)
	}
	if err := validateProxy("metadata.proxy", metadata.Proxy, nil, false); err != nil {
		return err
	}

	return nil
}

func validateProxy(
	field string,
	proxy *config.HTTPProxy,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if proxy == nil || proxyhelper.IsExplicitDisable(proxy) {
		return nil
	}
	if proxy.Auth != nil {
		if err := validateProxyAuth(field+".auth", proxy.Auth, credentials, strictCredentialRefs); err != nil {
			return err
		}
	}
	if _, err := proxyhelper.Build(field, proxy); err != nil {
		return err
	}
	return nil
}

func validateProxyAuth(
	field string,
	auth *config.ProxyAuth,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	hasBasic := auth.Basic != nil
	if countSet(hasBasic) != 1 {
		return faults.NewValidationError(field+" must define basic", nil)
	}
	if hasBasic {
		if err := validateCredentialRef(field+".basic.credentialsRef", auth.Basic.CredentialsRef, credentials, strictCredentialRefs); err != nil {
			return err
		}
	}
	return nil
}

func validateCredentials(items []config.Credential) (map[string]config.Credential, error) {
	index := make(map[string]config.Credential, len(items))
	for idx, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, faults.NewValidationError(fmt.Sprintf("credentials[%d].name is required", idx), nil)
		}
		if _, exists := index[name]; exists {
			return nil, faults.NewValidationError(fmt.Sprintf("duplicate credential name %q", name), nil)
		}
		if err := validateCredentialValue("credentials["+fmt.Sprint(idx)+"].username", item.Username); err != nil {
			return nil, err
		}
		if err := validateCredentialValue("credentials["+fmt.Sprint(idx)+"].password", item.Password); err != nil {
			return nil, err
		}
		item.Name = name
		index[name] = item
	}
	return index, nil
}

func validateCredentialValue(field string, value config.CredentialValue) error {
	switch {
	case value.Prompt == nil:
		if value.Literal() == "" {
			return faults.NewValidationError(field+" is required", nil)
		}
	case !value.Prompt.Prompt:
		return faults.NewValidationError(field+".prompt must be true when prompt configuration is used", nil)
	}
	return nil
}

func validateCredentialRef(
	field string,
	ref *config.CredentialsRef,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if ref == nil || strings.TrimSpace(ref.Name) == "" {
		return faults.NewValidationError(field+" is required", nil)
	}
	if strictCredentialRefs {
		if _, ok := credentials[strings.TrimSpace(ref.Name)]; !ok {
			return faults.NewValidationError(field+fmt.Sprintf(" references undefined credential %q", strings.TrimSpace(ref.Name)), nil)
		}
	}
	return nil
}

func applyOverrides(cfg config.Context, overrides map[string]string) (config.Context, error) {
	for _, key := range sortedOverrideKeys(overrides) {
		value := overrides[key]
		switch key {
		case "repository.git.local.baseDir":
			if cfg.Repository.Git == nil {
				return config.Context{}, faults.NewValidationError("override repository.git.local.baseDir requires repository.git to be configured", nil)
			}
			cfg.Repository.Git.Local.BaseDir = value
		case "repository.filesystem.baseDir":
			if cfg.Repository.Filesystem == nil {
				return config.Context{}, faults.NewValidationError("override repository.filesystem.baseDir requires repository.filesystem to be configured", nil)
			}
			cfg.Repository.Filesystem.BaseDir = value
		case "managedService.http.url":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managedService.http.url requires managedService.http to be configured", nil)
			}
			cfg.ManagedService.HTTP.BaseURL = value
		case "managedService.http.healthCheck":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managedService.http.healthCheck requires managedService.http to be configured", nil)
			}
			cfg.ManagedService.HTTP.HealthCheck = value
		case "managedService.http.proxy.http":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managedService.http.proxy.http requires managedService.http to be configured", nil)
			}
			if cfg.ManagedService.HTTP.Proxy == nil {
				cfg.ManagedService.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedService.HTTP.Proxy.HTTPURL = value
		case "managedService.http.proxy.https":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managedService.http.proxy.https requires managedService.http to be configured", nil)
			}
			if cfg.ManagedService.HTTP.Proxy == nil {
				cfg.ManagedService.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedService.HTTP.Proxy.HTTPSURL = value
		case "managedService.http.proxy.noProxy":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.NewValidationError("override managedService.http.proxy.noProxy requires managedService.http to be configured", nil)
			}
			if cfg.ManagedService.HTTP.Proxy == nil {
				cfg.ManagedService.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedService.HTTP.Proxy.NoProxy = value
		case "metadata.baseDir":
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
		case "metadata.bundleFile":
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
