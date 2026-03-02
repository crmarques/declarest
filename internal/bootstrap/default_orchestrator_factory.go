package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	bundlemetadata "github.com/crmarques/declarest/internal/providers/metadata/bundle"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	"github.com/crmarques/declarest/internal/defaultorch"
	httpgateway "github.com/crmarques/declarest/internal/providers/gateway/http"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/gateway"
)

func buildDefaultOrchestrator(
	ctx context.Context,
	contextService config.ContextService,
	selection config.ContextSelection,
) (*defaultorch.DefaultOrchestrator, error) {
	if contextService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return nil, err
	}

	metadataSource, err := resolveMetadataSource(ctx, resolvedContext)
	if err != nil {
		return nil, err
	}

	var metadataService metadata.MetadataService
	if metadataSource.BaseDir != "" {
		metadataService = fsmetadata.NewFSMetadataService(
			metadataSource.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
		if strings.TrimSpace(metadataSource.DeprecatedWarning) != "" {
			_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", metadataSource.DeprecatedWarning)
		}
	}

	var repo repository.ResourceStore
	switch {
	case resolvedContext.Repository.Filesystem != nil:
		repo = fsstore.NewLocalResourceRepository(
			resolvedContext.Repository.Filesystem.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
	case resolvedContext.Repository.Git != nil:
		repo = gitrepository.NewGitResourceRepository(
			*resolvedContext.Repository.Git,
			resolvedContext.Repository.ResourceFormat,
		)
	}

	var srv gateway.ResourceGateway
	if resolvedContext.ResourceServer != nil {
		if resolvedContext.ResourceServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "resource server provider is invalid", nil)
		}

		serverConfig := *resolvedContext.ResourceServer.HTTP
		serverConfig.OpenAPI = effectiveOpenAPISource(serverConfig.OpenAPI, metadataSource.OpenAPI)

		serverFormat := resolvedContext.Repository.ResourceFormat
		if serverFormat == "" {
			serverFormat = config.ResourceFormatJSON
		}
		serverOptions := []httpgateway.GatewayOption{
			httpgateway.WithResourceFormat(serverFormat),
		}
		if renderer, ok := metadataService.(metadata.ResourceOperationSpecRenderer); ok {
			serverOptions = append(serverOptions, httpgateway.WithMetadataRenderer(renderer))
		}
		serverManager, err := httpgateway.NewHTTPResourceGateway(
			serverConfig,
			serverOptions...,
		)
		if err != nil {
			return nil, err
		}
		srv = serverManager
	}

	var sec secrets.SecretProvider
	if resolvedContext.SecretStore != nil {
		switch {
		case resolvedContext.SecretStore.File != nil:
			secretService, err := filesecrets.NewFileSecretService(*resolvedContext.SecretStore.File)
			if err != nil {
				return nil, err
			}
			sec = secretService
		case resolvedContext.SecretStore.Vault != nil:
			secretService, err := vaultsecrets.NewVaultSecretService(*resolvedContext.SecretStore.Vault)
			if err != nil {
				return nil, err
			}
			sec = secretService
		default:
			return nil, faults.NewTypedError(faults.InternalError, "secret store provider is invalid", nil)
		}
	}

	return defaultorch.NewDefaultOrchestrator(
		repo,
		metadataService,
		srv,
		sec,
		resolvedContext.Repository.ResourceFormat,
	), nil
}

func effectiveOpenAPISource(configOpenAPI string, metadataOpenAPI string) string {
	if strings.TrimSpace(configOpenAPI) != "" {
		return configOpenAPI
	}
	return strings.TrimSpace(metadataOpenAPI)
}

type metadataSourceResolution struct {
	BaseDir           string
	OpenAPI           string
	DeprecatedWarning string
}

func resolveMetadataSource(ctx context.Context, context config.Context) (metadataSourceResolution, error) {
	if strings.TrimSpace(context.Metadata.Bundle) != "" {
		resolution, err := bundlemetadata.ResolveBundle(ctx, context.Metadata.Bundle)
		if err != nil {
			return metadataSourceResolution{}, err
		}
		return metadataSourceResolution{
			BaseDir:           resolution.MetadataDir,
			OpenAPI:           resolution.OpenAPI,
			DeprecatedWarning: resolution.DeprecatedWarning,
		}, nil
	}

	if context.Metadata.BaseDir != "" {
		return metadataSourceResolution{BaseDir: context.Metadata.BaseDir}, nil
	}

	switch {
	case context.Repository.Filesystem != nil:
		return metadataSourceResolution{BaseDir: context.Repository.Filesystem.BaseDir}, nil
	case context.Repository.Git != nil:
		return metadataSourceResolution{BaseDir: context.Repository.Git.Local.BaseDir}, nil
	default:
		return metadataSourceResolution{}, nil
	}
}
