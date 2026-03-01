package orchestrator

import (
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

var _ Orchestrator = (*DefaultOrchestrator)(nil)

type DefaultOrchestrator struct {
	repository repository.ResourceStore
	metadata   metadata.MetadataService
	server     server.ResourceServer
	secrets    secrets.SecretProvider

	resourceFormat string
}

func NewDefaultOrchestrator(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv server.ResourceServer,
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

func (r *DefaultOrchestrator) MetadataService() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadata
}

func (r *DefaultOrchestrator) ResourceServer() server.ResourceServer {
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
