package bootstrap

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	internalorchestrator "github.com/crmarques/declarest/internal/orchestrator"
	httpmanagedserver "github.com/crmarques/declarest/internal/providers/managedserver/http"
	bundlemetadata "github.com/crmarques/declarest/internal/providers/metadata/bundle"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

func buildOrchestrator(
	ctx context.Context,
	contextService config.ContextService,
	selection config.ContextSelection,
) (*internalorchestrator.Orchestrator, error) {
	if contextService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return nil, err
	}

	return buildOrchestratorFromResolvedContext(ctx, resolvedContext)
}

func buildOrchestratorFromResolvedContext(
	ctx context.Context,
	resolvedContext config.Context,
) (*internalorchestrator.Orchestrator, error) {
	emitSecurityWarnings(os.Stderr, resolvedContext)

	metadataSource, err := resolveMetadataSource(ctx, resolvedContext)
	if err != nil {
		return nil, err
	}

	repoBaseDir := resolvedRepositoryBaseDir(resolvedContext)
	var metadataService metadata.MetadataService
	if metadataSource.BaseDir != "" {
		switch {
		case repoBaseDir != "" && filepath.Clean(repoBaseDir) != filepath.Clean(metadataSource.BaseDir):
			metadataService = fsmetadata.NewLayeredFSMetadataService(
				metadataSource.BaseDir,
				repoBaseDir,
				metadataSource.WriteTarget,
			)
		default:
			metadataService = fsmetadata.NewFSMetadataService(metadataSource.BaseDir)
		}
		if strings.TrimSpace(metadataSource.DeprecatedWarning) != "" {
			cliutil.WriteStatusLine(os.Stderr, "WARNING", metadataSource.DeprecatedWarning)
		}
	}

	var repo repository.ResourceStore
	switch {
	case resolvedContext.Repository.Filesystem != nil:
		repo = fsstore.NewLocalResourceRepository(resolvedContext.Repository.Filesystem.BaseDir)
	case resolvedContext.Repository.Git != nil:
		repo = gitrepository.NewGitResourceRepository(*resolvedContext.Repository.Git)
	}

	var srv managedserver.ManagedServerClient
	if resolvedContext.ManagedServer != nil {
		if resolvedContext.ManagedServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "managed server provider is invalid", nil)
		}

		serverConfig := *resolvedContext.ManagedServer.HTTP
		serverConfig.OpenAPI = effectiveOpenAPISource(serverConfig.OpenAPI, metadataSource.OpenAPI)

		serverOptions := []httpmanagedserver.ClientOption{}
		if renderer, ok := metadataService.(metadata.ResourceOperationSpecRenderer); ok {
			serverOptions = append(serverOptions, httpmanagedserver.WithMetadataRenderer(renderer))
		}
		serverManager, err := httpmanagedserver.NewClient(
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

	return internalorchestrator.New(
		repo,
		metadataService,
		srv,
		sec,
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
	WriteTarget       fsmetadata.LayeredMetadataWriteTarget
}

func resolvedRepositoryBaseDir(ctx config.Context) string {
	switch {
	case ctx.Repository.Filesystem != nil:
		return strings.TrimSpace(ctx.Repository.Filesystem.BaseDir)
	case ctx.Repository.Git != nil:
		return strings.TrimSpace(ctx.Repository.Git.Local.BaseDir)
	default:
		return ""
	}
}

func emitSecurityWarnings(w io.Writer, resolvedContext config.Context) {
	if resolvedContext.ManagedServer != nil && resolvedContext.ManagedServer.HTTP != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedContext.ManagedServer.HTTP.BaseURL)), "http://") {
			cliutil.WriteStatusLine(w, "WARNING", "managed-server.http.base-url uses plain HTTP, credentials will be transmitted in cleartext")
		}
		if resolvedContext.ManagedServer.HTTP.Auth != nil &&
			resolvedContext.ManagedServer.HTTP.Auth.OAuth2 != nil &&
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedContext.ManagedServer.HTTP.Auth.OAuth2.TokenURL)), "http://") {
			cliutil.WriteStatusLine(w, "WARNING", "managed-server.http.auth.oauth2.token-url uses plain HTTP, client credentials will be transmitted in cleartext")
		}
		if resolvedContext.ManagedServer.HTTP.TLS != nil && resolvedContext.ManagedServer.HTTP.TLS.InsecureSkipVerify {
			cliutil.WriteStatusLine(w, "WARNING", "managed-server.http.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled")
		}
	}

	if resolvedContext.SecretStore != nil && resolvedContext.SecretStore.Vault != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedContext.SecretStore.Vault.Address)), "http://") {
			cliutil.WriteStatusLine(w, "WARNING", "secret-store.vault.address uses plain HTTP, credentials will be transmitted in cleartext")
		}
		if resolvedContext.SecretStore.Vault.TLS != nil && resolvedContext.SecretStore.Vault.TLS.InsecureSkipVerify {
			cliutil.WriteStatusLine(w, "WARNING", "secret-store.vault.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled")
		}
	}

	if resolvedContext.Repository.Git != nil && resolvedContext.Repository.Git.Remote != nil {
		if resolvedContext.Repository.Git.Remote.TLS != nil && resolvedContext.Repository.Git.Remote.TLS.InsecureSkipVerify {
			cliutil.WriteStatusLine(w, "WARNING", "repository.git.remote.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled")
		}
		if resolvedContext.Repository.Git.Remote.Auth != nil &&
			resolvedContext.Repository.Git.Remote.Auth.SSH != nil &&
			resolvedContext.Repository.Git.Remote.Auth.SSH.InsecureIgnoreHostKey {
			cliutil.WriteStatusLine(w, "WARNING", "repository.git.remote.auth.ssh.insecure-ignore-host-key is enabled, SSH host key verification is disabled")
		}
	}
}

func resolveMetadataSource(ctx context.Context, context config.Context) (metadataSourceResolution, error) {
	bundleRef := strings.TrimSpace(context.Metadata.Bundle)
	if bundleRef == "" {
		bundleRef = strings.TrimSpace(context.Metadata.BundleFile)
	}
	if bundleRef != "" {
		resolution, err := bundlemetadata.ResolveBundle(
			ctx,
			bundleRef,
			bundlemetadata.WithProxyConfig(context.Metadata.Proxy),
		)
		if err != nil {
			return metadataSourceResolution{}, err
		}
		return metadataSourceResolution{
			BaseDir:           resolution.MetadataDir,
			OpenAPI:           resolution.OpenAPI,
			DeprecatedWarning: resolution.DeprecatedWarning,
			WriteTarget:       fsmetadata.LayeredMetadataWriteLocal,
		}, nil
	}

	if context.Metadata.BaseDir != "" {
		return metadataSourceResolution{
			BaseDir:     context.Metadata.BaseDir,
			WriteTarget: fsmetadata.LayeredMetadataWriteShared,
		}, nil
	}

	switch {
	case context.Repository.Filesystem != nil:
		return metadataSourceResolution{
			BaseDir:     context.Repository.Filesystem.BaseDir,
			WriteTarget: fsmetadata.LayeredMetadataWriteShared,
		}, nil
	case context.Repository.Git != nil:
		return metadataSourceResolution{
			BaseDir:     context.Repository.Git.Local.BaseDir,
			WriteTarget: fsmetadata.LayeredMetadataWriteShared,
		}, nil
	default:
		return metadataSourceResolution{}, nil
	}
}
