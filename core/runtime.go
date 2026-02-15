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
)

func BuildExecutionRuntime(ctx context.Context, contextService config.ContextService, selection config.ContextSelection) (ExecutionRuntime, error) {
	if contextService == nil {
		return ExecutionRuntime{}, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return ExecutionRuntime{}, err
	}

	runtime := ExecutionRuntime{
		Name:        resolvedContext.Name,
		Environment: copyStringMap(selection.Overrides),
		Metadata: fsmetadata.NewFSMetadataService(
			resolveMetadataBaseDir(resolvedContext),
			resolvedContext.Repository.ResourceFormat,
		),
	}

	switch {
	case resolvedContext.Repository.Filesystem != nil:
		runtime.Repository = fsrepository.NewFSResourceRepository(
			resolvedContext.Repository.Filesystem.BaseDir,
			resolvedContext.Repository.ResourceFormat,
		)
	case resolvedContext.Repository.Git != nil:
		runtime.Repository = gitrepository.NewGitResourceRepository(
			*resolvedContext.Repository.Git,
			resolvedContext.Repository.ResourceFormat,
		)
	default:
		return ExecutionRuntime{}, faults.NewTypedError(faults.InternalError, "context repository provider is invalid", nil)
	}

	if resolvedContext.ManagedServer != nil {
		if resolvedContext.ManagedServer.HTTP == nil {
			return ExecutionRuntime{}, faults.NewTypedError(faults.InternalError, "managed server provider is invalid", nil)
		}
		serverManager, err := httpserver.NewHTTPResourceServerGateway(*resolvedContext.ManagedServer.HTTP)
		if err != nil {
			return ExecutionRuntime{}, err
		}
		runtime.Server = serverManager
	}

	if resolvedContext.SecretStore != nil {
		switch {
		case resolvedContext.SecretStore.File != nil:
			runtime.Secrets = &filesecrets.FileSecretService{}
		case resolvedContext.SecretStore.Vault != nil:
			runtime.Secrets = &vaultsecrets.VaultSecretService{}
		default:
			return ExecutionRuntime{}, faults.NewTypedError(faults.InternalError, "secret store provider is invalid", nil)
		}
	}

	return runtime, nil
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
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
