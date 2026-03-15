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

package promptauth

import "github.com/crmarques/declarest/config"

const (
	TargetRepositoryGitRemoteAuth      = "repository_git_remote_auth"
	TargetRepositoryGitRemoteProxyAuth = "repository_git_remote_proxy_auth"
	TargetManagedServerHTTPAuth        = "managed_server_http_auth"
	TargetManagedServerHTTPProxyAuth   = "managed_server_http_proxy_auth"
	TargetSecretStoreVaultAuth         = "secret_store_vault_auth"
	TargetSecretStoreVaultProxyAuth    = "secret_store_vault_proxy_auth"
	TargetMetadataProxyAuth            = "metadata_proxy_auth"
)

type Target struct {
	Key   string
	Label string
}

func BuildTargets(cfg config.Context) []Target {
	targets := make([]Target, 0, 7)

	if cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil {
		if cfg.Repository.Git.Remote.Auth != nil && cfg.Repository.Git.Remote.Auth.Prompt != nil {
			targets = append(targets, target(TargetRepositoryGitRemoteAuth))
		}
		if cfg.Repository.Git.Remote.Proxy != nil &&
			cfg.Repository.Git.Remote.Proxy.Auth != nil &&
			cfg.Repository.Git.Remote.Proxy.Auth.Prompt != nil {
			targets = append(targets, target(TargetRepositoryGitRemoteProxyAuth))
		}
	}

	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		if cfg.ManagedServer.HTTP.Auth != nil && cfg.ManagedServer.HTTP.Auth.Prompt != nil {
			targets = append(targets, target(TargetManagedServerHTTPAuth))
		}
		if cfg.ManagedServer.HTTP.Proxy != nil &&
			cfg.ManagedServer.HTTP.Proxy.Auth != nil &&
			cfg.ManagedServer.HTTP.Proxy.Auth.Prompt != nil {
			targets = append(targets, target(TargetManagedServerHTTPProxyAuth))
		}
	}

	if cfg.SecretStore != nil && cfg.SecretStore.Vault != nil {
		if cfg.SecretStore.Vault.Auth != nil && cfg.SecretStore.Vault.Auth.Prompt != nil {
			targets = append(targets, target(TargetSecretStoreVaultAuth))
		}
		if cfg.SecretStore.Vault.Proxy != nil &&
			cfg.SecretStore.Vault.Proxy.Auth != nil &&
			cfg.SecretStore.Vault.Proxy.Auth.Prompt != nil {
			targets = append(targets, target(TargetSecretStoreVaultProxyAuth))
		}
	}

	if cfg.Metadata.Proxy != nil &&
		cfg.Metadata.Proxy.Auth != nil &&
		cfg.Metadata.Proxy.Auth.Prompt != nil {
		targets = append(targets, target(TargetMetadataProxyAuth))
	}

	return targets
}

func target(key string) Target {
	switch key {
	case TargetRepositoryGitRemoteAuth:
		return Target{Key: key, Label: "git remote auth"}
	case TargetRepositoryGitRemoteProxyAuth:
		return Target{Key: key, Label: "git remote proxy auth"}
	case TargetManagedServerHTTPAuth:
		return Target{Key: key, Label: "managed-server auth"}
	case TargetManagedServerHTTPProxyAuth:
		return Target{Key: key, Label: "managed-server proxy auth"}
	case TargetSecretStoreVaultAuth:
		return Target{Key: key, Label: "vault auth"}
	case TargetSecretStoreVaultProxyAuth:
		return Target{Key: key, Label: "vault proxy auth"}
	case TargetMetadataProxyAuth:
		return Target{Key: key, Label: "metadata proxy auth"}
	default:
		return Target{Key: key, Label: key}
	}
}
