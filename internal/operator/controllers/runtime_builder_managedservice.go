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

package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func populateManagedServiceConfig(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	managedService *declarestv1alpha1.ManagedService,
	cfg *config.HTTPServer,
	openAPIPath string,
	cacheDir string,
	cleanup *cleanupRegistry,
) error {
	cfg.BaseURL = strings.TrimSpace(managedService.Spec.HTTP.BaseURL)
	cfg.HealthCheck = strings.TrimSpace(managedService.Spec.HTTP.HealthCheck)
	cfg.DefaultHeaders = managedService.Spec.HTTP.DefaultHeaders
	cfg.OpenAPI = strings.TrimSpace(openAPIPath)
	if managedService.Spec.HTTP.RequestThrottling != nil {
		throttling := managedService.Spec.HTTP.RequestThrottling
		cfg.RequestThrottling = &config.HTTPRequestThrottling{
			MaxConcurrentRequests: int(throttling.MaxConcurrentRequests),
			QueueSize:             int(throttling.QueueSize),
			RequestsPerSecond:     float64(throttling.RequestsPerSecond),
			Burst:                 int(throttling.Burst),
			ScopeKey:              fmt.Sprintf("%s/%s", managedService.Namespace, managedService.Name),
		}
	}

	if managedService.Spec.HTTP.TLS != nil {
		tlsConfig := &config.TLS{InsecureSkipVerify: managedService.Spec.HTTP.TLS.InsecureSkipVerify}
		if managedService.Spec.HTTP.TLS.CACertRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, managedService.Spec.HTTP.TLS.CACertRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "managed-service-tls"), "ca-cert", value)
			if err != nil {
				return err
			}
			tlsConfig.CACertFile = path
		}
		if managedService.Spec.HTTP.TLS.ClientCertRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, managedService.Spec.HTTP.TLS.ClientCertRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "managed-service-tls"), "client-cert", value)
			if err != nil {
				return err
			}
			tlsConfig.ClientCertFile = path
		}
		if managedService.Spec.HTTP.TLS.ClientKeyRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, managedService.Spec.HTTP.TLS.ClientKeyRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "managed-service-tls"), "client-key", value)
			if err != nil {
				return err
			}
			tlsConfig.ClientKeyFile = path
		}
		cfg.TLS = tlsConfig
	}

	proxy, err := resolveManagedServiceProxyConfig(ctx, reader, namespace, managedService.Spec.HTTP.Proxy)
	if err != nil {
		return err
	}
	if proxy != nil {
		cfg.Proxy = proxy
	}

	auth := &config.HTTPAuth{}
	if managedService.Spec.HTTP.Auth.OAuth2 != nil {
		oauth2 := managedService.Spec.HTTP.Auth.OAuth2
		clientID, err := readSecretValue(ctx, reader, namespace, oauth2.ClientIDRef)
		if err != nil {
			return err
		}
		clientSecret, err := readSecretValue(ctx, reader, namespace, oauth2.ClientSecretRef)
		if err != nil {
			return err
		}
		oauthConfig := &config.OAuth2{
			TokenURL:     oauth2.TokenURL,
			GrantType:    oauth2.GrantType,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scope:        oauth2.Scope,
			Audience:     oauth2.Audience,
		}
		if oauth2.UsernameRef != nil {
			username, err := readSecretValue(ctx, reader, namespace, oauth2.UsernameRef)
			if err != nil {
				return err
			}
			oauthConfig.Username = username
		}
		if oauth2.PasswordRef != nil {
			password, err := readSecretValue(ctx, reader, namespace, oauth2.PasswordRef)
			if err != nil {
				return err
			}
			oauthConfig.Password = password
		}
		auth.OAuth2 = oauthConfig
	}
	if managedService.Spec.HTTP.Auth.BasicAuth != nil {
		username, err := readSecretValue(ctx, reader, namespace, managedService.Spec.HTTP.Auth.BasicAuth.UsernameRef)
		if err != nil {
			return err
		}
		password, err := readSecretValue(ctx, reader, namespace, managedService.Spec.HTTP.Auth.BasicAuth.PasswordRef)
		if err != nil {
			return err
		}
		auth.Basic = &config.BasicAuth{
			Username: config.LiteralCredential(username),
			Password: config.LiteralCredential(password),
		}
	}
	if len(managedService.Spec.HTTP.Auth.CustomHeaders) > 0 {
		headers := make([]config.HeaderTokenAuth, 0, len(managedService.Spec.HTTP.Auth.CustomHeaders))
		for _, item := range managedService.Spec.HTTP.Auth.CustomHeaders {
			value, err := readSecretValue(ctx, reader, namespace, item.ValueRef)
			if err != nil {
				return err
			}
			headers = append(headers, config.HeaderTokenAuth{Header: item.Header, Prefix: item.Prefix, Value: value})
		}
		auth.CustomHeaders = headers
	}
	cfg.Auth = auth
	return nil
}

func resolveManagedServiceProxyConfig(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	proxySpec *declarestv1alpha1.HTTPProxySpec,
) (*config.HTTPProxy, error) {
	if proxySpec == nil {
		return nil, nil
	}

	proxy := &config.HTTPProxy{
		HTTPURL:  proxySpec.HTTPURL,
		HTTPSURL: proxySpec.HTTPSURL,
		NoProxy:  proxySpec.NoProxy,
	}
	if proxySpec.Auth == nil {
		return proxy, nil
	}

	username, err := readSecretValue(ctx, reader, namespace, proxySpec.Auth.UsernameRef)
	if err != nil {
		return nil, err
	}
	password, err := readSecretValue(ctx, reader, namespace, proxySpec.Auth.PasswordRef)
	if err != nil {
		return nil, err
	}
	proxy.Auth = &config.ProxyAuth{
		Basic: &config.BasicAuth{
			Username: config.LiteralCredential(username),
			Password: config.LiteralCredential(password),
		},
	}
	return proxy, nil
}
