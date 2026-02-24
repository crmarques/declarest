package resourceserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	serverdomain "github.com/crmarques/declarest/server"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "resource-server",
		Short: "Inspect resource-server connectivity and auth",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(deps),
		newCheckCommand(deps),
	)

	return command
}

func newGetCommand(deps common.CommandDependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "get",
		Short: "Read resource-server configuration or auth values",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetBaseURLCommand(deps),
		newGetTokenURLCommand(deps),
		newGetAccessTokenCommand(deps),
	)

	return command
}

func newGetBaseURLCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "base-url",
		Short: "Print resource-server HTTP base URL",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			httpConfig, err := resolveHTTPServerConfig(command.Context(), deps)
			if err != nil {
				return err
			}

			baseURL := strings.TrimSpace(httpConfig.BaseURL)
			if baseURL == "" {
				return common.ValidationError("resource-server.http.base-url is not configured", nil)
			}
			_, err = io.WriteString(command.OutOrStdout(), baseURL+"\n")
			return err
		},
	}
}

func newGetTokenURLCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "token-url",
		Short: "Print resource-server OAuth2 token URL",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			httpConfig, err := resolveHTTPServerConfig(command.Context(), deps)
			if err != nil {
				return err
			}

			if httpConfig.Auth == nil || httpConfig.Auth.OAuth2 == nil {
				return common.ValidationError("resource-server.http.auth.oauth2 is not configured", nil)
			}
			tokenURL := strings.TrimSpace(httpConfig.Auth.OAuth2.TokenURL)
			if tokenURL == "" {
				return common.ValidationError("resource-server.http.auth.oauth2.token-url is not configured", nil)
			}
			_, err = io.WriteString(command.OutOrStdout(), tokenURL+"\n")
			return err
		},
	}
}

func newGetAccessTokenCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "access-token",
		Short: "Fetch OAuth2 access token from the resource server",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			resourceServer, err := common.RequireResourceServer(deps)
			if err != nil {
				return err
			}

			provider, ok := resourceServer.(serverdomain.AccessTokenProvider)
			if !ok {
				return common.ValidationError(
					"resource-server get access-token requires resource-server.http.auth.oauth2 configuration",
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

func resolveHTTPServerConfig(ctx context.Context, deps common.CommandDependencies) (configdomain.HTTPServer, error) {
	contexts, err := common.RequireContexts(deps)
	if err != nil {
		return configdomain.HTTPServer{}, err
	}

	resolvedContext, err := contexts.ResolveContext(ctx, configdomain.ContextSelection{
		Name: strings.TrimSpace(common.ContextName(ctx)),
	})
	if err != nil {
		return configdomain.HTTPServer{}, err
	}
	if resolvedContext.ResourceServer == nil || resolvedContext.ResourceServer.HTTP == nil {
		return configdomain.HTTPServer{}, common.ValidationError("resource-server.http is not configured", nil)
	}

	return *resolvedContext.ResourceServer.HTTP, nil
}

func newCheckCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check resource-server connectivity",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			remoteReader, err := common.RequireRemoteReader(deps)
			if err != nil {
				return err
			}

			_, err = remoteReader.ListRemote(command.Context(), "/", orchestratordomain.ListPolicy{Recursive: false})
			if err == nil {
				_, writeErr := io.WriteString(command.OutOrStdout(), "resource-server check: OK (probe succeeded)\n")
				return writeErr
			}

			switch typedCategory(err) {
			case faults.NotFoundError, faults.ValidationError, faults.ConflictError:
				_, writeErr := io.WriteString(
					command.OutOrStdout(),
					fmt.Sprintf("resource-server check: OK (probe reached server and returned %s)\n", typedCategory(err)),
				)
				if writeErr != nil {
					return writeErr
				}
				_, writeErr = io.WriteString(command.OutOrStdout(), strings.TrimSpace(err.Error())+"\n")
				return writeErr
			default:
				return err
			}
		},
	}
}

func typedCategory(err error) faults.ErrorCategory {
	if err == nil {
		return ""
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return ""
	}
	return typedErr.Category
}
