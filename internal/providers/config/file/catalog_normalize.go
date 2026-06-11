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
	"sort"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

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

func applyOverrides(cfg config.Context, overrides map[string]string) (config.Context, error) {
	for _, key := range sortedOverrideKeys(overrides) {
		value := overrides[key]
		switch key {
		case "repository.git.local.baseDir":
			if cfg.Repository.Git == nil {
				return config.Context{}, faults.Invalid("override repository.git.local.baseDir requires repository.git to be configured", nil)
			}
			cfg.Repository.Git.Local.BaseDir = value
		case "repository.filesystem.baseDir":
			if cfg.Repository.Filesystem == nil {
				return config.Context{}, faults.Invalid("override repository.filesystem.baseDir requires repository.filesystem to be configured", nil)
			}
			cfg.Repository.Filesystem.BaseDir = value
		case "managedService.http.url":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.Invalid("override managedService.http.url requires managedService.http to be configured", nil)
			}
			cfg.ManagedService.HTTP.BaseURL = value
		case "managedService.http.healthCheck":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.Invalid("override managedService.http.healthCheck requires managedService.http to be configured", nil)
			}
			cfg.ManagedService.HTTP.HealthCheck = value
		case "managedService.http.proxy.http":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.Invalid("override managedService.http.proxy.http requires managedService.http to be configured", nil)
			}
			if cfg.ManagedService.HTTP.Proxy == nil {
				cfg.ManagedService.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedService.HTTP.Proxy.HTTPURL = value
		case "managedService.http.proxy.https":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.Invalid("override managedService.http.proxy.https requires managedService.http to be configured", nil)
			}
			if cfg.ManagedService.HTTP.Proxy == nil {
				cfg.ManagedService.HTTP.Proxy = &config.HTTPProxy{}
			}
			cfg.ManagedService.HTTP.Proxy.HTTPSURL = value
		case "managedService.http.proxy.noProxy":
			if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
				return config.Context{}, faults.Invalid("override managedService.http.proxy.noProxy requires managedService.http to be configured", nil)
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
