package context

import (
	"errors"
	"fmt"
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/openapi"
	"declarest/internal/reconciler"
	"declarest/internal/repository"
	"declarest/internal/secrets"
)

func buildReconcilerFromConfig(cfg *ContextConfig) (reconciler.Reconciler, error) {
	if cfg == nil {
		return nil, errors.New("context configuration is missing")
	}

	var (
		baseDir         string
		metadataBaseDir string
		repoManager     repository.ResourceRepositoryManager
	)

	resourceFormat := repository.ResourceFormatJSON
	if cfg.Repository != nil {
		parsed, err := repository.ParseResourceFormat(cfg.Repository.ResourceFormat)
		if err != nil {
			return nil, err
		}
		resourceFormat = parsed
	}

	if cfg.Repository != nil {
		if cfg.Repository.Git != nil && cfg.Repository.Filesystem != nil {
			return nil, errors.New("repository configuration must define either git or filesystem, not both")
		}

		if cfg.Repository.Filesystem != nil {
			baseDir = cfg.Repository.Filesystem.BaseDir
			repoManager = repository.NewFileSystemResourceRepositoryManager(baseDir)
		} else if cfg.Repository.Git != nil {
			if cfg.Repository.Git.Local != nil {
				baseDir = cfg.Repository.Git.Local.BaseDir
			}
			repoManager = repository.NewGitResourceRepositoryManager(baseDir)
		}
	}

	if cfg.Metadata != nil {
		metadataBaseDir = strings.TrimSpace(cfg.Metadata.BaseDir)
	}
	if metadataBaseDir == "" {
		metadataBaseDir = baseDir
	}

	if repoManager == nil {
		repoManager = repository.NewGitResourceRepositoryManager("")
	}

	if setter, ok := repoManager.(interface{ SetMetadataBaseDir(string) }); ok {
		setter.SetMetadataBaseDir(metadataBaseDir)
	}

	if cfg.Repository != nil && cfg.Repository.Git != nil {
		if setter, ok := repoManager.(interface {
			SetConfig(*repository.GitResourceRepositoryConfig)
		}); ok {
			setter.SetConfig(cfg.Repository.Git)
		}
	}

	if setter, ok := repoManager.(interface {
		SetResourceFormat(repository.ResourceFormat)
	}); ok {
		setter.SetResourceFormat(resourceFormat)
	}

	var serverManager managedserver.ResourceServerManager
	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		serverManager = managedserver.NewHTTPResourceServerManager(cfg.ManagedServer.HTTP)
	}

	var openapiSpec *openapi.Spec
	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		openapiSource := strings.TrimSpace(cfg.ManagedServer.HTTP.OpenAPI)
		if openapiSource != "" {
			httpManager, ok := serverManager.(*managedserver.HTTPResourceServerManager)
			if !ok || httpManager == nil {
				return nil, errors.New("openapi configuration requires an http managed server")
			}
			data, err := httpManager.LoadOpenAPISpec(openapiSource)
			if err != nil {
				return nil, err
			}
			spec, err := openapi.ParseSpec(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse openapi spec %q: %w", openapiSource, err)
			}
			openapiSpec = spec
		}
	}

	var secretsManager secrets.SecretsManager
	if cfg.SecretManager != nil {
		if cfg.SecretManager.File != nil && cfg.SecretManager.Vault != nil {
			return nil, errors.New("secret store configuration must define either file or vault, not both")
		}
		switch {
		case cfg.SecretManager.File != nil:
			secretsManager = secrets.NewFileSecretsManager(cfg.SecretManager.File)
		case cfg.SecretManager.Vault != nil:
			secretsManager = secrets.NewVaultSecretsManager(cfg.SecretManager.Vault)
		default:
			return nil, errors.New("secret store configuration is required")
		}
	}

	recon := &reconciler.DefaultReconciler{
		ResourceServerManager:     serverManager,
		ResourceRepositoryManager: repoManager,
		SecretsManager:            secretsManager,
	}

	provider := repository.NewDefaultResourceRecordProvider(metadataBaseDir, recon)
	provider.SetResourceFormat(resourceFormat)
	if openapiSpec != nil {
		provider.SetOpenAPISpec(openapiSpec)
	}
	recon.ResourceRecordProvider = provider

	return recon, nil
}
