package orchestrator

import (
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

var _ orchestrator.Orchestrator = (*Orchestrator)(nil)

type Orchestrator struct {
	repository repository.ResourceStore
	metadata   metadata.MetadataService
	server     managedserver.ManagedServerClient
	secrets    secrets.SecretProvider
}

func New(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv managedserver.ManagedServerClient,
	sec secrets.SecretProvider,
) *Orchestrator {
	return &Orchestrator{
		repository: repo,
		metadata:   meta,
		server:     srv,
		secrets:    sec,
	}
}

func (r *Orchestrator) RepositoryStore() repository.ResourceStore {
	if r == nil {
		return nil
	}
	return r.repository
}

func (r *Orchestrator) RepositorySync() repository.RepositorySync {
	if r == nil || r.repository == nil {
		return nil
	}
	if sync, ok := r.repository.(repository.RepositorySync); ok {
		return sync
	}
	return nil
}

func (r *Orchestrator) MetadataService() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadata
}

func (r *Orchestrator) ManagedServerClient() managedserver.ManagedServerClient {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *Orchestrator) SecretProvider() secrets.SecretProvider {
	if r == nil {
		return nil
	}
	return r.secrets
}
