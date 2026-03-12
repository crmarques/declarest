package cliutil

import (
	"github.com/crmarques/declarest/config"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type CommandDependencies = appdeps.Dependencies

func NewCommandDependencies(
	orch orchestrator.Orchestrator,
	contexts config.ContextService,
	services orchestrator.ServiceAccessor,
) CommandDependencies {
	return appdeps.Dependencies{
		Orchestrator: orch,
		Contexts:     contexts,
		Services:     services,
	}
}

func RequireContexts(deps CommandDependencies) (config.ContextService, error) {
	return appdeps.RequireContexts(deps)
}

func RequireOrchestrator(deps CommandDependencies) (orchestrator.Orchestrator, error) {
	return appdeps.RequireOrchestrator(deps)
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
	return appdeps.RequireResourceStore(deps)
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

func RequireMetadataService(deps CommandDependencies) (metadatadomain.MetadataService, error) {
	return appdeps.RequireMetadataService(deps)
}

func RequireSecretProvider(deps CommandDependencies) (secretdomain.SecretProvider, error) {
	return appdeps.RequireSecretProvider(deps)
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
