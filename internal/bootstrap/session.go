package bootstrap

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
)

func NewContextService(opts BootstrapConfig) config.ContextService {
	return configfile.NewService(opts.ContextCatalogPath)
}

func NewSession(opts BootstrapConfig, selection config.ContextSelection) (Session, error) {
	contextService := NewContextService(opts)
	orch, err := buildOrchestrator(context.Background(), contextService, selection)
	if err != nil {
		return Session{}, err
	}

	return Session{
		Contexts:     contextService,
		Orchestrator: orch,
		Services:     orch,
	}, nil
}

func NewSessionFromResolvedContext(resolvedContext config.Context) (Session, error) {
	orch, err := buildOrchestratorFromResolvedContext(context.Background(), resolvedContext)
	if err != nil {
		return Session{}, err
	}
	return Session{
		Contexts:     nil,
		Orchestrator: orch,
		Services:     orch,
	}, nil
}
