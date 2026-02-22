package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	localfs "github.com/crmarques/declarest/internal/providers/repository/localfs"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
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

	defaultOrchestrator := &orchestrator.DefaultOrchestrator{
		Metadata: fsmetadata.NewFSMetadataService(
			resolveMetadataBaseDir(resolvedContext),
			resolvedContext.Repository.ResourceFormat,
		),
	}

	switch {
	case resolvedContext.Repository.Filesystem != nil:
		defaultOrchestrator.Repository = localfs.NewLocalResourceRepository(
			resolvedContext.Repository.Filesystem.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
	case resolvedContext.Repository.Git != nil:
		defaultOrchestrator.Repository = gitrepository.NewGitResourceRepository(
			*resolvedContext.Repository.Git,
			resolvedContext.Repository.ResourceFormat,
		)
	default:
		return nil, faults.NewTypedError(faults.InternalError, "context repository provider is invalid", nil)
	}

	if resolvedContext.ResourceServer != nil {
		if resolvedContext.ResourceServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "resource server provider is invalid", nil)
		}
		serverManager, err := httpserver.NewHTTPResourceServerGateway(*resolvedContext.ResourceServer.HTTP)
		if err != nil {
			return nil, err
		}
		serverManager.SetMetadataService(defaultOrchestrator.Metadata)
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
