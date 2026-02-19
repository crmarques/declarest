package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
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

	return DeclarestContext{
		Contexts:     contextService,
		Orchestrator: defaultOrchestrator,
		Repository:   defaultOrchestrator.Repository,
		Metadata:     defaultOrchestrator.Metadata,
		Secrets:      defaultOrchestrator.Secrets,
	}, nil
}
