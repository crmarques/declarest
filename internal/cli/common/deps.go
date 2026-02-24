package common

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

type CommandDependencies struct {
	Orchestrator   orchestrator.Orchestrator
	Contexts       config.ContextService
	ResourceStore  repository.ResourceStore
	RepositorySync repository.RepositorySync
	Metadata       metadata.MetadataService
	Secrets        secrets.SecretProvider
	ResourceServer server.ResourceServer
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
	if deps.ResourceStore == nil {
		return nil, ValidationError("resource store is not configured", nil)
	}
	return deps.ResourceStore, nil
}

func RequireRepositorySync(deps CommandDependencies) (repository.RepositorySync, error) {
	if deps.RepositorySync == nil {
		return nil, ValidationError("repository sync is not configured", nil)
	}
	return deps.RepositorySync, nil
}

func RequireMetadataService(deps CommandDependencies) (metadata.MetadataService, error) {
	if deps.Metadata == nil {
		return nil, ValidationError("metadata service is not configured", nil)
	}
	return deps.Metadata, nil
}

func RequireSecretProvider(deps CommandDependencies) (secrets.SecretProvider, error) {
	if deps.Secrets == nil {
		return nil, ValidationError("secret provider is not configured", nil)
	}
	return deps.Secrets, nil
}

func RequireResourceServer(deps CommandDependencies) (server.ResourceServer, error) {
	if deps.ResourceServer == nil {
		return nil, ValidationError("resource server is not configured", nil)
	}
	return deps.ResourceServer, nil
}
