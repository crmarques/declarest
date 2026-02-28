package core

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
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
)

func buildDefaultOrchestrator(
	ctx context.Context,
	contextService config.ContextService,
	selection config.ContextSelection,
) (*orchestrator.DefaultOrchestrator, error) {
	if contextService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return nil, err
	}

	defaultOrchestrator := &orchestrator.DefaultOrchestrator{}
	defaultOrchestrator.SetResourceFormat(resolvedContext.Repository.ResourceFormat)

	metadataSource, err := resolveMetadataSource(ctx, resolvedContext)
	if err != nil {
		return nil, err
	}
	if metadataSource.BaseDir != "" {
		defaultOrchestrator.Metadata = fsmetadata.NewFSMetadataService(
			metadataSource.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
		if strings.TrimSpace(metadataSource.DeprecatedWarning) != "" {
			_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", metadataSource.DeprecatedWarning)
		}
	}

	switch {
	case resolvedContext.Repository.Filesystem != nil:
		defaultOrchestrator.Repository = fsstore.NewLocalResourceRepository(
			resolvedContext.Repository.Filesystem.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
	case resolvedContext.Repository.Git != nil:
		defaultOrchestrator.Repository = gitrepository.NewGitResourceRepository(
			*resolvedContext.Repository.Git,
			resolvedContext.Repository.ResourceFormat,
		)
	}

	if resolvedContext.ResourceServer != nil {
		if resolvedContext.ResourceServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "resource server provider is invalid", nil)
		}

		serverConfig := *resolvedContext.ResourceServer.HTTP
		if strings.TrimSpace(serverConfig.OpenAPI) == "" && strings.TrimSpace(metadataSource.OpenAPI) != "" {
			serverConfig.OpenAPI = metadataSource.OpenAPI
		}

		serverFormat := resolvedContext.Repository.ResourceFormat
		if serverFormat == "" {
			serverFormat = config.ResourceFormatJSON
		}
		serverOptions := []httpserver.GatewayOption{
			httpserver.WithResourceFormat(serverFormat),
		}
		if renderer, ok := defaultOrchestrator.Metadata.(metadata.ResourceOperationSpecRenderer); ok {
			serverOptions = append(serverOptions, httpserver.WithMetadataRenderer(renderer))
		}
		serverManager, err := httpserver.NewHTTPResourceServerGateway(
			serverConfig,
			serverOptions...,
		)
		if err != nil {
			return nil, err
		}
		defaultOrchestrator.Server = serverManager
	}

	if resolvedContext.SecretStore != nil {
		switch {
		case resolvedContext.SecretStore.File != nil:
			secretService, err := filesecrets.NewFileSecretService(*resolvedContext.SecretStore.File)
			if err != nil {
				return nil, err
			}
			defaultOrchestrator.Secrets = secretService
		case resolvedContext.SecretStore.Vault != nil:
			secretService, err := vaultsecrets.NewVaultSecretService(*resolvedContext.SecretStore.Vault)
			if err != nil {
				return nil, err
			}
			defaultOrchestrator.Secrets = secretService
		default:
			return nil, faults.NewTypedError(faults.InternalError, "secret store provider is invalid", nil)
		}
	}

	return defaultOrchestrator, nil
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
