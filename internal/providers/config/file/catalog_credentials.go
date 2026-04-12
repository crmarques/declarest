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
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func injectContextCredentials(
	cfg config.Context,
	credentials map[string]config.Credential,
) (config.Context, error) {
	if cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil {
		if cfg.Repository.Git.Remote.Auth != nil && cfg.Repository.Git.Remote.Auth.Basic != nil {
			if err := injectBasicCredentials(
				"repository.git.remote.auth.basic.credentialsRef",
				cfg.Repository.Git.Remote.Auth.Basic,
				credentials,
			); err != nil {
				return config.Context{}, err
			}
		}
		if err := injectProxyCredentials(cfg.Repository.Git.Remote.Proxy, "repository.git.remote.proxy.auth.basic.credentialsRef", credentials); err != nil {
			return config.Context{}, err
		}
	}

	if cfg.ManagedService != nil && cfg.ManagedService.HTTP != nil {
		if cfg.ManagedService.HTTP.Auth != nil && cfg.ManagedService.HTTP.Auth.Basic != nil {
			if err := injectBasicCredentials(
				"managedService.http.auth.basic.credentialsRef",
				cfg.ManagedService.HTTP.Auth.Basic,
				credentials,
			); err != nil {
				return config.Context{}, err
			}
		}
		if err := injectProxyCredentials(cfg.ManagedService.HTTP.Proxy, "managedService.http.proxy.auth.basic.credentialsRef", credentials); err != nil {
			return config.Context{}, err
		}
	}

	if cfg.SecretStore != nil && cfg.SecretStore.Vault != nil {
		if cfg.SecretStore.Vault.Auth != nil && cfg.SecretStore.Vault.Auth.Password != nil {
			if err := injectVaultUserPasswordCredentials(
				cfg.SecretStore.Vault.Auth.Password,
				"secretStore.vault.auth.password.credentialsRef",
				credentials,
			); err != nil {
				return config.Context{}, err
			}
		}
		if err := injectProxyCredentials(cfg.SecretStore.Vault.Proxy, "secretStore.vault.proxy.auth.basic.credentialsRef", credentials); err != nil {
			return config.Context{}, err
		}
	}

	if err := injectProxyCredentials(cfg.Metadata.Proxy, "metadata.proxy.auth.basic.credentialsRef", credentials); err != nil {
		return config.Context{}, err
	}

	return cfg, nil
}

func injectProxyCredentials(
	proxy *config.HTTPProxy,
	field string,
	credentials map[string]config.Credential,
) error {
	if proxy == nil || proxy.Auth == nil || proxy.Auth.Basic == nil {
		return nil
	}
	return injectBasicCredentials(field, proxy.Auth.Basic, credentials)
}

func injectVaultUserPasswordCredentials(
	target *config.VaultUserPasswordAuth,
	field string,
	credentials map[string]config.Credential,
) error {
	if target == nil {
		return nil
	}

	item, err := referencedCredential(field, target.CredentialsRef, credentials)
	if err != nil {
		return err
	}
	target.Username = item.Username
	target.Password = item.Password
	return nil
}

func injectBasicCredentials(
	field string,
	target *config.BasicAuth,
	credentials map[string]config.Credential,
) error {
	if target == nil {
		return nil
	}

	item, err := referencedCredential(field, target.CredentialsRef, credentials)
	if err != nil {
		return err
	}
	target.Username = item.Username
	target.Password = item.Password
	return nil
}

func referencedCredential(
	field string,
	ref *config.CredentialsRef,
	credentials map[string]config.Credential,
) (config.Credential, error) {
	if ref == nil || strings.TrimSpace(ref.Name) == "" {
		return config.Credential{}, faults.NewValidationError(field+" is required", nil)
	}
	name := strings.TrimSpace(ref.Name)
	item, ok := credentials[name]
	if !ok {
		return config.Credential{}, faults.NewValidationError(
			fmt.Sprintf("%s references undefined credential %q", field, name),
			nil,
		)
	}
	return item, nil
}

func mergeContextCredentials(
	catalog config.ContextCatalog,
	credentials map[string]config.Credential,
) (config.ContextCatalog, error) {
	if len(credentials) == 0 {
		return catalog, nil
	}

	index, err := validateCredentials(catalog.Credentials)
	if err != nil {
		return config.ContextCatalog{}, err
	}

	keys := make([]string, 0, len(credentials))
	for key := range credentials {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		item := credentials[key]
		if strings.TrimSpace(item.Name) == "" {
			item.Name = key
		}
		singleIndex, err := validateCredentials([]config.Credential{item})
		if err != nil {
			return config.ContextCatalog{}, err
		}
		for name, normalized := range singleIndex {
			if existing, ok := index[name]; ok {
				if !credentialsEqual(existing, normalized) {
					return config.ContextCatalog{}, faults.NewValidationError(
						fmt.Sprintf("credential %q already exists with different content", name),
						nil,
					)
				}
				continue
			}
			catalog.Credentials = append(catalog.Credentials, normalized)
			index[name] = normalized
		}
	}

	return catalog, nil
}

func credentialsEqual(a, b config.Credential) bool {
	return strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name) &&
		a.Username.Literal() == b.Username.Literal() &&
		a.Username.IsPrompt() == b.Username.IsPrompt() &&
		a.Username.PersistInSession() == b.Username.PersistInSession() &&
		a.Password.Literal() == b.Password.Literal() &&
		a.Password.IsPrompt() == b.Password.IsPrompt() &&
		a.Password.PersistInSession() == b.Password.PersistInSession()
}
