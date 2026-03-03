package bootstrap

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
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

	return Session{
		Contexts:     contextService,
		Orchestrator: defaultOrchestrator,
		Services:     defaultOrchestrator,
	}, nil
}

func NewSessionFromResolvedContext(resolvedContext config.Context) (Session, error) {
	defaultOrchestrator, err := buildDefaultOrchestratorFromResolvedContext(context.Background(), resolvedContext)
	if err != nil {
		return Session{}, err
	}
	return Session{
		Contexts:     nil,
		Orchestrator: defaultOrchestrator,
		Services:     defaultOrchestrator,
	}, nil
}
