package core

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

func NewDeclarestContext(opts BootstrapConfig, selection config.ContextSelection) (DeclarestContext, error) {
	contextService := NewContextService(opts)
	defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, selection)

	if err != nil {
		return DeclarestContext{}, err
	}

	var repositorySync repository.RepositorySync
	if defaultOrchestrator.Repository != nil {
		var ok bool
		repositorySync, ok = defaultOrchestrator.Repository.(repository.RepositorySync)
		if !ok {
			return DeclarestContext{}, faults.NewTypedError(
				faults.InternalError,
				"repository provider does not implement sync capabilities",
				nil,
			)
		}
	}

	return DeclarestContext{
		Contexts:       contextService,
		Orchestrator:   defaultOrchestrator,
		ResourceStore:  defaultOrchestrator.Repository,
		RepositorySync: repositorySync,
		Metadata:       defaultOrchestrator.Metadata,
		Secrets:        defaultOrchestrator.Secrets,
		ResourceServer: defaultOrchestrator.Server,
	}, nil
}
