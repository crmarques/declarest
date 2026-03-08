package cliutil

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

type CommandDependencies struct {
	Orchestrator orchestrator.Orchestrator
	Contexts     config.ContextService
	Services     orchestrator.ServiceAccessor
}

func NewCommandDependencies(
	orch orchestrator.Orchestrator,
	contexts config.ContextService,
	services orchestrator.ServiceAccessor,
) CommandDependencies {
	return CommandDependencies{
		Orchestrator: orch,
		Contexts:     contexts,
		Services:     services,
	}
}

func RequireContexts(deps CommandDependencies) (config.ContextService, error) {
	if deps.Contexts == nil {
		return nil, ValidationError("context service is not configured", nil)
	}
	return deps.Contexts, nil
}

func RequireOrchestrator(deps CommandDependencies) (orchestrator.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, ValidationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func RequireCompletionService(deps CommandDependencies) (orchestrator.CompletionService, error) {
	orchestratorService, err := RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}
	return orchestratorService, nil
}

func RequireRemoteReader(deps CommandDependencies) (orchestrator.RemoteReader, error) {
	orchestratorService, err := RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}
	return orchestratorService, nil
}

func RequireResourceStore(deps CommandDependencies) (repository.ResourceStore, error) {
	if deps.Services == nil {
		return nil, ValidationError("resource store is not configured", nil)
	}
	store := deps.Services.RepositoryStore()
	if store == nil {
		return nil, ValidationError("resource store is not configured", nil)
	}
	return store, nil
}

func RequireRepositorySync(deps CommandDependencies) (repository.RepositorySync, error) {
	if deps.Services == nil {
		return nil, ValidationError("repository sync is not configured", nil)
	}
	sync := deps.Services.RepositorySync()
	if sync == nil {
		return nil, ValidationError("repository sync is not configured", nil)
	}
	return sync, nil
}

func RequireMetadataService(deps CommandDependencies) (metadata.MetadataService, error) {
	if deps.Services == nil {
		return nil, ValidationError("metadata service is not configured", nil)
	}
	md := deps.Services.MetadataService()
	if md == nil {
		return nil, ValidationError("metadata service is not configured", nil)
	}
	return md, nil
}

func RequireSecretProvider(deps CommandDependencies) (secrets.SecretProvider, error) {
	if deps.Services == nil {
		return nil, ValidationError("secret provider is not configured", nil)
	}
	sp := deps.Services.SecretProvider()
	if sp == nil {
		return nil, ValidationError("secret provider is not configured", nil)
	}
	return sp, nil
}

func RequireManagedServerClient(deps CommandDependencies) (managedserver.ManagedServerClient, error) {
	if deps.Services == nil {
		return nil, ValidationError("managed server client is not configured", nil)
	}
	client := deps.Services.ManagedServerClient()
	if client == nil {
		return nil, ValidationError("managed server client is not configured", nil)
	}
	return client, nil
}
