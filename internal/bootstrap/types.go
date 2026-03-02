package bootstrap

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/orchestrator"
)

type Session struct {
	Contexts     config.ContextService
	Orchestrator orchestrator.Orchestrator
	Services     orchestrator.ServiceAccessor
}

type BootstrapConfig struct {
	ContextCatalogPath string
}
