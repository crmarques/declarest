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

	provider := repository.NewDefaultResourceRecordProvider(baseDir, recon)
	recon.ResourceRecordProvider = provider

	return recon, nil
}
