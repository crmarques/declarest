package common

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/reconciler"
)

type CommandWiring struct {
	Reconciler reconciler.ResourceReconciler
	Contexts   config.ContextService
}

func RequireContexts(deps CommandWiring) (config.ContextService, error) {
	if deps.Contexts == nil {
		return nil, ValidationError("context service is not configured", nil)
	}
	return deps.Contexts, nil
}

func RequireReconciler(deps CommandWiring) (reconciler.ResourceReconciler, error) {
	if deps.Reconciler == nil {
		return nil, ValidationError("reconciler is not configured", nil)
	}
	return deps.Reconciler, nil
}
