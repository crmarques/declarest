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

package managedservice

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	"github.com/spf13/cobra"
)

func NewCommand(deps cliutil.CommandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "server",
		Short: "Inspect server connectivity and auth",
		Args:  cobra.NoArgs,
	}
	commandmeta.MarkRequiresContextBootstrap(command)

	command.AddCommand(
		newGetCommand(deps),
		newCheckCommand(deps),
	)

	return command
}

func newGetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "get",
		Short: "Read server configuration or auth values",
		Args:  cobra.NoArgs,
	}

	baseURLCommand := newGetBaseURLCommand(deps)
	tokenURLCommand := newGetTokenURLCommand(deps)
	accessTokenCommand := newGetAccessTokenCommand(deps)
	commandmeta.MarkTextOnlyOutput(baseURLCommand)
	commandmeta.MarkTextOnlyOutput(tokenURLCommand)
	commandmeta.MarkTextOnlyOutput(accessTokenCommand)

	command.AddCommand(
		baseURLCommand,
		tokenURLCommand,
		accessTokenCommand,
	)

	return command
}

func newGetBaseURLCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "base-url",
		Short: "Print managed-service HTTP base URL",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			httpConfig, err := resolveHTTPServerConfig(command.Context(), deps)
			if err != nil {
				return err
			}

			baseURL := strings.TrimSpace(httpConfig.BaseURL)
			if baseURL == "" {
				return cliutil.ValidationError("managed-service.http.base-url is not configured", nil)
			}
			_, err = io.WriteString(command.OutOrStdout(), baseURL+"\n")
			return err
		},
	}
}

func newGetTokenURLCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "token-url",
		Short: "Print managed-service OAuth2 token URL",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			httpConfig, err := resolveHTTPServerConfig(command.Context(), deps)
			if err != nil {
				return err
			}

			if httpConfig.Auth == nil || httpConfig.Auth.OAuth2 == nil {
				return cliutil.ValidationError("managed-service.http.auth.oauth2 is not configured", nil)
			}
			tokenURL := strings.TrimSpace(httpConfig.Auth.OAuth2.TokenURL)
			if tokenURL == "" {
				return cliutil.ValidationError("managed-service.http.auth.oauth2.token-url is not configured", nil)
			}
			_, err = io.WriteString(command.OutOrStdout(), tokenURL+"\n")
			return err
		},
	}
}

func newGetAccessTokenCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "access-token",
		Short: "Fetch OAuth2 access token from the managed service",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			managedServiceClient, err := cliutil.RequireManagedServiceClient(deps)
			if err != nil {
				return err
			}

			provider, ok := managedServiceClient.(managedservicedomain.AccessTokenProvider)
			if !ok {
				return cliutil.ValidationError(
					"server get access-token requires managed-service.http.auth.oauth2 configuration",
					nil,
				)
			}

			token, err := provider.GetAccessToken(command.Context())
			if err != nil {
				return err
			}

			_, err = io.WriteString(command.OutOrStdout(), token+"\n")
			return err
		},
	}
}

func resolveHTTPServerConfig(ctx context.Context, deps cliutil.CommandDependencies) (configdomain.HTTPServer, error) {
	contexts, err := cliutil.RequireContexts(deps)
	if err != nil {
		return configdomain.HTTPServer{}, err
	}

	resolvedContext, err := contexts.ResolveContext(ctx, configdomain.ContextSelection{
		Name: strings.TrimSpace(cliutil.ContextName(ctx)),
	})
	if err != nil {
		return configdomain.HTTPServer{}, err
	}
	if resolvedContext.ManagedService == nil || resolvedContext.ManagedService.HTTP == nil {
		return configdomain.HTTPServer{}, cliutil.ValidationError("managed-service.http is not configured", nil)
	}

	return *resolvedContext.ManagedService.HTTP, nil
}

func newCheckCommand(deps cliutil.CommandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "check",
		Short: "Check managed-service connectivity",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			httpConfig, err := resolveHTTPServerConfig(command.Context(), deps)
			if err != nil {
				return err
			}

			managedServiceClient, err := cliutil.RequireManagedServiceClient(deps)
			if err != nil {
				return err
			}

			probePath, err := resolveHealthCheckProbePath(httpConfig)
			if err != nil {
				return err
			}

			if _, err := managedServiceClient.Request(command.Context(), managedservicedomain.RequestSpec{
				Method: http.MethodGet,
				Path:   probePath,
			}); err != nil {
				return err
			}

			_, writeErr := io.WriteString(
				command.OutOrStdout(),
				fmt.Sprintf("server check: OK (probe succeeded: GET %s)\n", renderHealthCheckTarget(httpConfig)),
			)
			return writeErr
		},
	}
	commandmeta.MarkTextOnlyOutput(command)
	return command
}

func renderHealthCheckTarget(httpConfig configdomain.HTTPServer) string {
	healthCheck := strings.TrimSpace(httpConfig.HealthCheck)
	if healthCheck != "" {
		return healthCheck
	}
	return strings.TrimSpace(httpConfig.BaseURL)
}

func resolveHealthCheckProbePath(httpConfig configdomain.HTTPServer) (string, error) {
	healthCheck := strings.TrimSpace(httpConfig.HealthCheck)
	if healthCheck == "" {
		baseURL := strings.TrimSpace(httpConfig.BaseURL)
		if baseURL == "" {
			return "/", nil
		}
		baseParsed, err := url.Parse(baseURL)
		if err != nil {
			return "", cliutil.ValidationError("managed-service.http.base-url is invalid", err)
		}
		basePath := strings.TrimSpace(baseParsed.Path)
		if basePath == "" {
			return "/", nil
		}
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		return basePath, nil
	}

	parsed, err := url.Parse(healthCheck)
	if err != nil {
		return "", cliutil.ValidationError("managed-service.http.health-check is invalid", err)
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return "", cliutil.ValidationError("managed-service.http.health-check must not include query parameters", nil)
	}

	// Relative paths are resolved against managed-service.http.base-url by the managed-service provider.
	if parsed.Scheme == "" && parsed.Host == "" {
		parsedPath := strings.TrimSpace(parsed.Path)
		if parsedPath == "" {
			return "", cliutil.ValidationError("managed-service.http.health-check is invalid", nil)
		}
		if !strings.HasPrefix(parsedPath, "/") {
			parsedPath = "/" + parsedPath
		}
		return parsedPath, nil
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", cliutil.ValidationError("managed-service.http.health-check URL must use http or https", nil)
	}
	if parsed.Host == "" {
		return "", cliutil.ValidationError("managed-service.http.health-check URL host is required", nil)
	}

	baseURL, err := url.Parse(strings.TrimSpace(httpConfig.BaseURL))
	if err != nil {
		return "", cliutil.ValidationError("managed-service.http.base-url is invalid", err)
	}
	if !strings.EqualFold(parsed.Scheme, baseURL.Scheme) || !strings.EqualFold(parsed.Host, baseURL.Host) {
		return "", cliutil.ValidationError(
			"managed-service.http.health-check URL must share scheme and host with managed-service.http.base-url",
			nil,
		)
	}

	basePath := baseURL.EscapedPath()
	if strings.TrimSpace(basePath) == "" {
		basePath = "/"
	}
	targetPath := parsed.EscapedPath()
	if strings.TrimSpace(targetPath) == "" {
		targetPath = "/"
	}

	relativePath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", cliutil.ValidationError("managed-service.http.health-check URL path is invalid", err)
	}
	relativePath = strings.ReplaceAll(relativePath, "\\", "/")
	if relativePath == "." {
		return "/", nil
	}

	return "/" + relativePath, nil
}
