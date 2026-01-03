package context

import (
	"errors"

	"declarest/internal/managedserver"
	"declarest/internal/reconciler"
	"declarest/internal/repository"
	"declarest/internal/secrets"
)

func buildReconcilerFromConfig(cfg *ContextConfig) (reconciler.Reconciler, error) {
	if cfg == nil {
		return nil, errors.New("context configuration is missing")
	}

	var (
		baseDir     string
		repoManager repository.ResourceRepositoryManager
	)

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

	if repoManager == nil {
		repoManager = repository.NewGitResourceRepositoryManager("")
	}

	if cfg.Repository != nil && cfg.Repository.Git != nil {
		if setter, ok := repoManager.(interface {
			SetConfig(*repository.GitResourceRepositoryConfig)
		}); ok {
			setter.SetConfig(cfg.Repository.Git)
		}
	}

	var serverManager managedserver.ResourceServerManager
	if cfg.ManagedServer != nil && cfg.ManagedServer.HTTP != nil {
		serverManager = managedserver.NewHTTPResourceServerManager(cfg.ManagedServer.HTTP)
	}

	var secretsManager secrets.SecretsManager
	if cfg.SecretManager != nil {
		if cfg.SecretManager.File == nil {
			return nil, errors.New("secret manager configuration is required")
		}
		secretsManager = secrets.NewFileSecretsManager(cfg.SecretManager.File)
	}

	recon := &reconciler.DefaultReconciler{
		ResourceServerManager:     serverManager,
		ResourceRepositoryManager: repoManager,
		SecretsManager:            secretsManager,
	}

	provider := repository.NewDefaultResourceRecordProvider(baseDir, recon)
	recon.ResourceRecordProvider = provider

	return recon, nil
}
