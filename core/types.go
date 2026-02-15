package core

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/reconciler"
)

type DeclarestContext struct {
	Contexts   config.ContextService
	Reconciler reconciler.ResourceReconciler
}

type BootstrapConfig struct {
	ContextCatalogPath string
}
