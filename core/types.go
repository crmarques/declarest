package core

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

type DeclarestContext struct {
	Contexts       config.ContextService
	Orchestrator   orchestrator.Orchestrator
	ResourceStore  repository.ResourceStore
	RepositorySync repository.RepositorySync
	Metadata       metadata.MetadataService
	Secrets        secrets.SecretProvider
	ResourceServer server.ResourceServer
}

type BootstrapConfig struct {
	ContextCatalogPath string
}
