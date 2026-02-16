package core

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

type DeclarestContext struct {
	Contexts     config.ContextService
	Orchestrator orchestrator.Orchestrator
	Repository   repository.ResourceRepository
	Metadata     metadata.MetadataService
	Secrets      secrets.SecretProvider
}

type BootstrapConfig struct {
	ContextCatalogPath string
}
