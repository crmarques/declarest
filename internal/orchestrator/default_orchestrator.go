package orchestrator

import (
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

var _ orchestrator.Orchestrator = (*DefaultOrchestrator)(nil)

type DefaultOrchestrator struct {
	repository repository.ResourceStore
	metadata   metadata.MetadataService
	server     managedserver.ManagedServerClient
	secrets    secrets.SecretProvider
}

func NewDefaultOrchestrator(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv managedserver.ManagedServerClient,
	sec secrets.SecretProvider,
) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		repository: repo,
		metadata:   meta,
		server:     srv,
		secrets:    sec,
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

func (r *DefaultOrchestrator) ManagedServerClient() managedserver.ManagedServerClient {
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
