package core

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

type DeclarestContext struct {
	// Repository is kept for compatibility while consumers migrate to
	// ResourceStore/RepositorySync split interfaces.
	Repository     repository.ResourceRepository
	Contexts       config.ContextService
	Orchestrator   orchestrator.Orchestrator
	ResourceStore  repository.ResourceStore
	RepositorySync repository.RepositorySync
	Metadata       metadata.MetadataService
	Secrets        secrets.SecretProvider
}

type BootstrapConfig struct {
	ContextCatalogPath string
}
