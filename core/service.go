package core

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	defaultreconciler "github.com/crmarques/declarest/internal/providers/reconciler/default"
	"github.com/crmarques/declarest/repository"
)

func NewAppState(opts BootstrapConfig) AppState {
	contextService := configfile.NewFileContextService(opts.ConfigFilePath)
	selection := config.ContextSelection{Name: opts.ContextName}

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
