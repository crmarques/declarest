package common

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/secrets"
)

type CommandWiring struct {
	Reconciler reconciler.ResourceReconciler
	Contexts   config.ContextService
}

type MetadataServiceProvider interface {
	MetadataManager() metadata.MetadataService
}

type SecretProviderManager interface {
	SecretManager() secrets.SecretProvider
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

func RequireMetadataService(deps CommandWiring) (metadata.MetadataService, error) {
	reconciler, err := RequireReconciler(deps)
	if err != nil {
		return nil, err
	}

	provider, ok := reconciler.(MetadataServiceProvider)
	if !ok {
		return nil, ValidationError("metadata service is not configured", nil)
	}

	service := provider.MetadataManager()
	if service == nil {
		return nil, ValidationError("metadata service is not configured", nil)
	}
	return service, nil
}

func RequireSecretProvider(deps CommandWiring) (secrets.SecretProvider, error) {
	reconciler, err := RequireReconciler(deps)
	if err != nil {
		return nil, err
	}

	provider, ok := reconciler.(SecretProviderManager)
	if !ok {
		return nil, ValidationError("secret provider is not configured", nil)
	}

	secretProvider := provider.SecretManager()
	if secretProvider == nil {
		return nil, ValidationError("secret provider is not configured", nil)
	}
	return secretProvider, nil
}
