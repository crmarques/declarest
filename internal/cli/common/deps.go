package common

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

type CommandDependencies struct {
	Orchestrator orchestrator.Orchestrator
	Contexts     config.ContextService
	Repository   repository.ResourceRepository
	Metadata     metadata.MetadataService
	Secrets      secrets.SecretProvider
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

func RequireRepository(deps CommandDependencies) (repository.ResourceRepository, error) {
	if deps.Repository == nil {
		return nil, ValidationError("repository is not configured", nil)
	}
	return deps.Repository, nil
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
