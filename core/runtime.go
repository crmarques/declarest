package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsrepository "github.com/crmarques/declarest/internal/providers/repository/fs"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
	"github.com/crmarques/declarest/reconciler"
)

func buildDefaultReconciler(
	ctx context.Context,
	contextService config.ContextService,
	selection config.ContextSelection,
) (*reconciler.DefaultReconciler, error) {
	if contextService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return nil, err
	}

	defaultReconciler := &reconciler.DefaultReconciler{
		Name: resolvedContext.Name,
		MetadataService: fsmetadata.NewFSMetadataService(
			resolveMetadataBaseDir(resolvedContext),
			resolvedContext.Repository.ResourceFormat,
		),
	}

	switch {
	case resolvedContext.Repository.Filesystem != nil:
		defaultReconciler.RepositoryManager = fsrepository.NewFSResourceRepository(
			resolvedContext.Repository.Filesystem.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
	case resolvedContext.Repository.Git != nil:
		defaultReconciler.RepositoryManager = gitrepository.NewGitResourceRepository(
			*resolvedContext.Repository.Git,
			resolvedContext.Repository.ResourceFormat,
		)
	default:
		return nil, faults.NewTypedError(faults.InternalError, "context repository provider is invalid", nil)
	}

	if resolvedContext.ManagedServer != nil {
		if resolvedContext.ManagedServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "managed server provider is invalid", nil)
		}
		serverManager, err := httpserver.NewHTTPResourceServerGateway(*resolvedContext.ManagedServer.HTTP)
		if err != nil {
			return nil, err
		}
		defaultReconciler.ServerManager = serverManager
	}

	if resolvedContext.SecretStore != nil {
		switch {
		case resolvedContext.SecretStore.File != nil:
			defaultReconciler.SecretsProvider = &filesecrets.FileSecretService{}
		case resolvedContext.SecretStore.Vault != nil:
			defaultReconciler.SecretsProvider = &vaultsecrets.VaultSecretService{}
		default:
			return nil, faults.NewTypedError(faults.InternalError, "secret store provider is invalid", nil)
		}
	}

	return defaultReconciler, nil
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
