package core

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

type AppState struct {
	Contexts   config.ContextService
	Reconciler reconciler.ResourceReconciler
}

type BootstrapConfig struct {
	ContextCatalogPath string
}

type ExecutionRuntime struct {
	Name        string
	Environment map[string]string
	Repository  repository.ResourceRepository
	Metadata    metadata.MetadataService
	Server      server.RemoteResourceGateway
	Secrets     secrets.SecretService
}
