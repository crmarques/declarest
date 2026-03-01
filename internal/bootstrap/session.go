package bootstrap

import (
	"context"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	"github.com/crmarques/declarest/repository"
)

func NewContextService(opts BootstrapConfig) config.ContextService {
	return configfile.NewFileContextService(opts.ContextCatalogPath)
}

func NewSession(opts BootstrapConfig, selection config.ContextSelection) (Session, error) {
	contextService := NewContextService(opts)
	defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, selection)

	if err != nil {
		return Session{}, err
	}

	var repositorySync repository.RepositorySync
	if defaultOrchestrator.RepositoryStore() != nil {
		var ok bool
		repositorySync, ok = defaultOrchestrator.RepositoryStore().(repository.RepositorySync)
		if !ok {
			return Session{}, faults.NewTypedError(
				faults.InternalError,
				"repository provider does not implement sync capabilities",
				nil,
			)
		}
	}

	return Session{
		Contexts:       contextService,
		Orchestrator:   defaultOrchestrator,
		ResourceStore:  defaultOrchestrator.RepositoryStore(),
		RepositorySync: repositorySync,
		Metadata:       defaultOrchestrator.MetadataService(),
		Secrets:        defaultOrchestrator.SecretProvider(),
		ResourceServer: defaultOrchestrator.ResourceServer(),
	}, nil
}
