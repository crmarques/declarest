package core

import (
	"context"

	"github.com/crmarques/declarest/config"
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

	var repositoryCompat repository.ResourceRepository
	if typed, ok := defaultOrchestrator.Repository.(repository.ResourceRepository); ok {
		repositoryCompat = typed
	}

	var repositorySync repository.RepositorySync
	if typed, ok := defaultOrchestrator.Repository.(repository.RepositorySync); ok {
		repositorySync = typed
	}

	return DeclarestContext{
		Repository:     repositoryCompat,
		Contexts:       contextService,
		Orchestrator:   defaultOrchestrator,
		ResourceStore:  defaultOrchestrator.Repository,
		RepositorySync: repositorySync,
		Metadata:       defaultOrchestrator.Metadata,
		Secrets:        defaultOrchestrator.Secrets,
	}, nil
}
