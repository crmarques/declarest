package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
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

	metadataBaseDir := resolveMetadataBaseDir(resolvedContext)
	if metadataBaseDir != "" {
		defaultOrchestrator.Metadata = fsmetadata.NewFSMetadataService(
			metadataBaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
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
			*resolvedContext.ResourceServer.HTTP,
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

func resolveMetadataBaseDir(context config.Context) string {
	if context.Metadata.BaseDir != "" {
		return context.Metadata.BaseDir
	}

	switch {
	case context.Repository.Filesystem != nil:
		return context.Repository.Filesystem.BaseDir
	case context.Repository.Git != nil:
		return context.Repository.Git.Local.BaseDir
	default:
		return ""
	}
}
