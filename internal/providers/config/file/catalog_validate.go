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
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/envref"
	"github.com/crmarques/declarest/faults"
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
			return faults.Invalid("currentContext must be empty when contexts list is empty", nil)
		}
		return nil
	}

	seen := map[string]struct{}{}
	for _, item := range contextCatalog.Contexts {
		if item.Name == "" {
			return faults.Invalid("context name must not be empty", nil)
		}
		if _, exists := seen[item.Name]; exists {
			return faults.Invalid(fmt.Sprintf("duplicate context name %q", item.Name), nil)
		}
		seen[item.Name] = struct{}{}

		if err := validateConfig(item, credentials, true); err != nil {
			return err
		}
	}

	if contextCatalog.CurrentContext == "" {
		return faults.Invalid("currentContext must be set when contexts are defined", nil)
	}

	if _, exists := seen[contextCatalog.CurrentContext]; !exists {
		return faults.Invalid(fmt.Sprintf("currentContext %q does not match any context", contextCatalog.CurrentContext), nil)
	}

	return nil
}

func validateResolvedCatalog(contextCatalog config.ContextCatalog) error {
	return validateCatalog(envref.ExpandExactEnvPlaceholders(contextCatalog))
}

func validateConfig(cfg config.Context, credentials map[string]config.Credential, strictCredentialRefs bool) error {
	cfg = normalizeConfig(cfg)

	if cfg.Name == "" {
		return faults.Invalid("context name must not be empty", nil)
	}
	if cfg.Repository.Git == nil && cfg.Repository.Filesystem == nil && cfg.ManagedService == nil {
		return faults.Invalid("context must define at least one of repository or managedService", nil)
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

func validateRepository(
	repository config.Repository,
	credentials map[string]config.Credential,
	strictCredentialRefs bool,
) error {
	if repository.Git == nil && repository.Filesystem == nil {
		return nil
	}

	if countSet(repository.Git != nil, repository.Filesystem != nil) != 1 {
		return faults.Invalid("repository must define exactly one of git or filesystem", nil)
	}

	if repository.Git != nil {
		if repository.Git.Local.BaseDir == "" {
			return faults.Invalid("repository.git.local.baseDir is required", nil)
		}
		if repository.Git.Remote != nil {
			if repository.Git.Remote.URL == "" {
				return faults.Invalid("repository.git.remote.url is required", nil)
			}
			if repository.Git.Remote.Auth != nil {
				if countSet(
					repository.Git.Remote.Auth.Basic != nil,
					repository.Git.Remote.Auth.SSH != nil,
					repository.Git.Remote.Auth.AccessKey != nil,
				) != 1 {
					return faults.Invalid("repository.git.remote.auth must define exactly one of basic, ssh, accessKey", nil)
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
		return faults.Invalid("repository.filesystem.baseDir is required", nil)
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
		return faults.Invalid("managedService must define http", nil)
	}
	if resourceServer.HTTP.BaseURL == "" {
		return faults.Invalid("managedService.http.url is required", nil)
	}
	if resourceServer.HTTP.Auth == nil {
		return faults.Invalid("managedService.http.auth is required", nil)
	}

	if countSet(
		resourceServer.HTTP.Auth.OAuth2 != nil,
		resourceServer.HTTP.Auth.Basic != nil,
		len(resourceServer.HTTP.Auth.CustomHeaders) > 0,
	) != 1 {
		return faults.Invalid("managedService.http.auth must define exactly one of oauth2, basic, customHeaders", nil)
	}

	if resourceServer.HTTP.Auth.OAuth2 != nil {
		oauth := resourceServer.HTTP.Auth.OAuth2
		if oauth.TokenURL == "" || oauth.GrantType == "" || oauth.ClientID == "" || oauth.ClientSecret == "" {
			return faults.Invalid("managedService.http.auth.oauth2 requires tokenURL, grantType, clientID, clientSecret", nil)
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
			return faults.Invalid(
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
		return faults.Invalid("managedService.http.requestThrottling must define at least one of maxConcurrentRequests or requestsPerSecond", nil)
	}
	if throttling.MaxConcurrentRequests < 0 {
		return faults.Invalid("managedService.http.requestThrottling.maxConcurrentRequests must be greater than zero when set", nil)
	}
	if throttling.QueueSize < 0 {
		return faults.Invalid("managedService.http.requestThrottling.queueSize must be greater than or equal to zero", nil)
	}
	if throttling.QueueSize > 0 && throttling.MaxConcurrentRequests <= 0 {
		return faults.Invalid("managedService.http.requestThrottling.queueSize requires maxConcurrentRequests", nil)
	}
	if throttling.RequestsPerSecond < 0 {
		return faults.Invalid("managedService.http.requestThrottling.requestsPerSecond must be greater than zero when set", nil)
	}
	if throttling.Burst < 0 {
		return faults.Invalid("managedService.http.requestThrottling.burst must be greater than zero when set", nil)
	}
	if throttling.Burst > 0 && throttling.RequestsPerSecond <= 0 {
		return faults.Invalid("managedService.http.requestThrottling.burst requires requestsPerSecond", nil)
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
		return faults.Invalid("managedService.http.healthCheck is invalid", err)
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return faults.Invalid("managedService.http.healthCheck must not include query parameters", nil)
	}

	// Relative paths are interpreted against managedService.http.url.
	if parsed.Scheme == "" && parsed.Host == "" {
		if strings.TrimSpace(parsed.Path) == "" {
			return faults.Invalid("managedService.http.healthCheck is invalid", nil)
		}
		return nil
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return faults.Invalid("managedService.http.healthCheck URL must use http or https", nil)
	}
	if parsed.Host == "" {
		return faults.Invalid("managedService.http.healthCheck URL host is required", nil)
	}

	_, err = filepath.Rel("/", parsed.Path)
	if err != nil {
		return faults.Invalid("managedService.http.healthCheck URL path is invalid", err)
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
		return faults.Invalid("secretStore must define exactly one of file or vault", nil)
	}

	if secretStore.File != nil {
		if secretStore.File.Path == "" {
			return faults.Invalid("secretStore.file.path is required", nil)
		}
		if countSet(
			secretStore.File.Key != "",
			secretStore.File.KeyFile != "",
			secretStore.File.Passphrase != "",
			secretStore.File.PassphraseFile != "",
		) != 1 {
			return faults.Invalid("secretStore.file must define exactly one of key, keyFile, passphrase, passphraseFile", nil)
		}
	}

	if secretStore.Vault != nil {
		if secretStore.Vault.Address == "" {
			return faults.Invalid("secretStore.vault.address is required", nil)
		}
		if secretStore.Vault.Auth == nil {
			return faults.Invalid("secretStore.vault.auth is required", nil)
		}
		if countSet(
			secretStore.Vault.Auth.Token != "",
			secretStore.Vault.Auth.Password != nil,
			secretStore.Vault.Auth.AppRole != nil,
		) != 1 {
			return faults.Invalid("secretStore.vault.auth must define exactly one of token, password, appRole", nil)
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
		return faults.Invalid("metadata must define at most one of baseDir, bundle, or bundleFile", nil)
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
		return faults.Invalid(field+" must define basic", nil)
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
			return nil, faults.Invalid(fmt.Sprintf("credentials[%d].name is required", idx), nil)
		}
		if _, exists := index[name]; exists {
			return nil, faults.Invalid(fmt.Sprintf("duplicate credential name %q", name), nil)
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
			return faults.Invalid(field+" is required", nil)
		}
	case !value.Prompt.Prompt:
		return faults.Invalid(field+".prompt must be true when prompt configuration is used", nil)
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
		return faults.Invalid(field+" is required", nil)
	}
	if strictCredentialRefs {
		if _, ok := credentials[strings.TrimSpace(ref.Name)]; !ok {
			return faults.Invalid(field+fmt.Sprintf(" references undefined credential %q", strings.TrimSpace(ref.Name)), nil)
		}
	}
	return nil
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
