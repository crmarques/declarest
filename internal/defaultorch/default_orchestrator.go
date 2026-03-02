package defaultorch

import (
	"github.com/crmarques/declarest/gateway"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

var _ orchestrator.Orchestrator = (*DefaultOrchestrator)(nil)

type DefaultOrchestrator struct {
	repository repository.ResourceStore
	metadata   metadata.MetadataService
	server     gateway.ResourceGateway
	secrets    secrets.SecretProvider

	resourceFormat string
}

func NewDefaultOrchestrator(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv gateway.ResourceGateway,
	sec secrets.SecretProvider,
	resourceFormat string,
) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		repository:     repo,
		metadata:       meta,
		server:         srv,
		secrets:        sec,
		resourceFormat: metadata.NormalizeResourceFormat(resourceFormat),
	}
}

func (r *DefaultOrchestrator) RepositoryStore() repository.ResourceStore {
	if r == nil {
		return nil
	}
	return r.repository
}

func (r *DefaultOrchestrator) RepositorySync() repository.RepositorySync {
	if r == nil || r.repository == nil {
		return nil
	}
	if sync, ok := r.repository.(repository.RepositorySync); ok {
		return sync
	}
	return nil
}

func (r *DefaultOrchestrator) MetadataService() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadata
}

func (r *DefaultOrchestrator) ResourceGateway() gateway.ResourceGateway {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *DefaultOrchestrator) SecretProvider() secrets.SecretProvider {
	if r == nil {
		return nil
	}
	return r.secrets
}

func (r *DefaultOrchestrator) effectiveResourceFormat() string {
	if r == nil {
		return metadata.NormalizeResourceFormat("")
	}
	return metadata.NormalizeResourceFormat(r.resourceFormat)
}
