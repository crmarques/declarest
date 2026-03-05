package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
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

func buildDefaultOrchestrator(
	ctx context.Context,
	contextService config.ContextService,
	selection config.ContextSelection,
) (*internalorchestrator.DefaultOrchestrator, error) {
	if contextService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "context service must not be nil", nil)
	}

	resolvedContext, err := contextService.ResolveContext(ctx, selection)
	if err != nil {
		return nil, err
	}

	return buildDefaultOrchestratorFromResolvedContext(ctx, resolvedContext)
}

func buildDefaultOrchestratorFromResolvedContext(
	ctx context.Context,
	resolvedContext config.Context,
) (*internalorchestrator.DefaultOrchestrator, error) {
	emitSecurityWarnings(os.Stderr, resolvedContext)

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

	var srv managedserver.ManagedServerClient
	if resolvedContext.ManagedServer != nil {
		if resolvedContext.ManagedServer.HTTP == nil {
			return nil, faults.NewTypedError(faults.InternalError, "managed server provider is invalid", nil)
		}

		serverConfig := *resolvedContext.ManagedServer.HTTP
		serverConfig.OpenAPI = effectiveOpenAPISource(serverConfig.OpenAPI, metadataSource.OpenAPI)

		serverFormat := resolvedContext.Repository.ResourceFormat
		if serverFormat == "" {
			serverFormat = config.ResourceFormatJSON
		}
		serverOptions := []httpmanagedserver.ManagedServerClientOption{
			httpmanagedserver.WithResourceFormat(serverFormat),
		}
		if renderer, ok := metadataService.(metadata.ResourceOperationSpecRenderer); ok {
			serverOptions = append(serverOptions, httpmanagedserver.WithMetadataRenderer(renderer))
		}
		serverManager, err := httpmanagedserver.NewHTTPManagedServerClient(
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

	return internalorchestrator.NewDefaultOrchestrator(
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

func emitSecurityWarnings(w io.Writer, resolvedContext config.Context) {
	if resolvedContext.ManagedServer != nil && resolvedContext.ManagedServer.HTTP != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedContext.ManagedServer.HTTP.BaseURL)), "http://") {
			_, _ = fmt.Fprintf(w, "warning: managed-server.http.base-url uses plain HTTP, credentials will be transmitted in cleartext\n")
		}
		if resolvedContext.ManagedServer.HTTP.TLS != nil && resolvedContext.ManagedServer.HTTP.TLS.InsecureSkipVerify {
			_, _ = fmt.Fprintf(w, "warning: managed-server.http.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled\n")
		}
	}

	if resolvedContext.SecretStore != nil && resolvedContext.SecretStore.Vault != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(resolvedContext.SecretStore.Vault.Address)), "http://") {
			_, _ = fmt.Fprintf(w, "warning: secret-store.vault.address uses plain HTTP, credentials will be transmitted in cleartext\n")
		}
		if resolvedContext.SecretStore.Vault.TLS != nil && resolvedContext.SecretStore.Vault.TLS.InsecureSkipVerify {
			_, _ = fmt.Fprintf(w, "warning: secret-store.vault.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled\n")
		}
	}

	if resolvedContext.Repository.Git != nil && resolvedContext.Repository.Git.Remote != nil {
		if resolvedContext.Repository.Git.Remote.TLS != nil && resolvedContext.Repository.Git.Remote.TLS.InsecureSkipVerify {
			_, _ = fmt.Fprintf(w, "warning: repository.git.remote.tls.insecure-skip-verify is enabled, TLS certificate verification is disabled\n")
		}
		if resolvedContext.Repository.Git.Remote.Auth != nil &&
			resolvedContext.Repository.Git.Remote.Auth.SSH != nil &&
			resolvedContext.Repository.Git.Remote.Auth.SSH.InsecureIgnoreHostKey {
			_, _ = fmt.Fprintf(w, "warning: repository.git.remote.auth.ssh.insecure-ignore-host-key is enabled, SSH host key verification is disabled\n")
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
