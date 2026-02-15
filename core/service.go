package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
)

func NewDeclarestContext(opts BootstrapConfig, selection config.ContextSelection) (DeclarestContext, error) {
	contextService := configfile.NewFileContextService(opts.ContextCatalogPath)
	defaultReconciler, err := buildDefaultReconciler(context.Background(), contextService, selection)

	if err != nil {
		return DeclarestContext{}, err
	}

	return DeclarestContext{
		Contexts:   contextService,
		Reconciler: defaultReconciler,
	}, nil
}
