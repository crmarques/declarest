package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	defaultreconciler "github.com/crmarques/declarest/internal/providers/reconciler/default"
	"github.com/crmarques/declarest/repository"
)

func NewAppState(opts BootstrapConfig, selection config.ContextSelection) AppState {
	contextService := configfile.NewFileContextService(opts.ContextCatalogPath)

	return AppState{
		Contexts: contextService,
		Reconciler: defaultreconciler.NewResourceReconciler(func(ctx context.Context) (repository.ResourceRepository, error) {
			runtime, err := BuildExecutionRuntime(ctx, contextService, selection)
			if err != nil {
				return nil, err
			}
			return runtime.Repository, nil
		}),
	}
}
