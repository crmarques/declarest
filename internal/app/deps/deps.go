package deps

import (
	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
	Contexts     configdomain.ContextService
	Repository   repository.ResourceStore
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
}

func RequireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, faults.NewValidationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func RequireContexts(deps Dependencies) (configdomain.ContextService, error) {
	if deps.Contexts == nil {
		return nil, faults.NewValidationError("context service is not configured", nil)
	}
	return deps.Contexts, nil
}

func RequireResourceStore(deps Dependencies) (repository.ResourceStore, error) {
	if deps.Repository == nil {
		return nil, faults.NewValidationError("resource repository is not configured", nil)
	}
	return deps.Repository, nil
}

func RequireMetadataService(deps Dependencies) (metadatadomain.MetadataService, error) {
	if deps.Metadata == nil {
		return nil, faults.NewValidationError("metadata service is not configured", nil)
	}
	return deps.Metadata, nil
}

func RequireSecretProvider(deps Dependencies) (secretdomain.SecretProvider, error) {
	if deps.Secrets == nil {
		return nil, faults.NewValidationError("secret provider is not configured", nil)
	}
	return deps.Secrets, nil
}
