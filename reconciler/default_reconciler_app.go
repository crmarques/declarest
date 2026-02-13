package reconciler

import (
	"errors"
	"net/http"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultReconciler) Close() error {
	if r == nil {
		return nil
	}
	var first error
	if r.ResourceRepositoryManager != nil {
		if err := r.ResourceRepositoryManager.Close(); err != nil {
			first = err
		}
	}
	if r.ResourceServerManager != nil {
		if err := r.ResourceServerManager.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.SecretsManager != nil {
		if err := r.SecretsManager.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *DefaultReconciler) ManagedServerConfigured() bool {
	return r != nil && r.ResourceServerManager != nil
}

func (r *DefaultReconciler) SecretsConfigured() bool {
	return r != nil && r.SecretsManager != nil
}

func (r *DefaultReconciler) CheckManagedServerAccess() error {
	if r == nil || r.ResourceServerManager == nil {
		return errors.New("resource server manager is not configured")
	}
	if checker, ok := r.ResourceServerManager.(managedserver.AccessChecker); ok {
		return checker.CheckAccess()
	}
	if err := r.ResourceServerManager.Init(); err != nil {
		return err
	}

	spec := managedserver.RequestSpec{
		Kind: managedserver.KindHTTP,
		HTTP: &managedserver.HTTPRequestSpec{
			Path: "/",
		},
	}

	_, err := r.ResourceServerManager.ResourceExists(spec)
	if err == nil {
		return nil
	}

	var httpErr *managedserver.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	return err
}

func (r *DefaultReconciler) RepositoryResourcePathsWithErrors() ([]string, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return nil, errors.New("resource repository manager is not configured")
	}
	if lister, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryPathLister); ok {
		return lister.ListResourcePathsWithErrors()
	}
	return r.ResourceRepositoryManager.ListResourcePaths(), nil
}

func (r *DefaultReconciler) SetSkipRepositorySync(skip bool) {
	if r == nil {
		return
	}
	r.SkipRepositorySync = skip
}

func (r *DefaultReconciler) ResourceRecord(path string) (resource.ResourceRecord, error) {
	if r == nil {
		return resource.ResourceRecord{}, errors.New("reconciler is nil")
	}
	return r.recordFor(path)
}

func (r *DefaultReconciler) MergedMetadata(path string) (resource.ResourceMetadata, error) {
	if r == nil || r.ResourceRecordProvider == nil {
		return resource.ResourceMetadata{}, errors.New("resource record provider is not configured")
	}
	if err := r.validateMetadataPath(path); err != nil {
		return resource.ResourceMetadata{}, err
	}
	return r.ResourceRecordProvider.GetMergedMetadata(path)
}

func (r *DefaultReconciler) MetadataChildCollections(baseSegments []string) ([]string, error) {
	if r == nil || r.ResourceRecordProvider == nil {
		return nil, nil
	}
	if provider, ok := r.ResourceRecordProvider.(metadata.ChildCollectionProvider); ok {
		return provider.MetadataChildCollections(baseSegments)
	}
	return nil, nil
}

func (r *DefaultReconciler) OpenAPISpec() *openapi.Spec {
	if r == nil || r.ResourceRecordProvider == nil {
		return nil
	}
	if provider, ok := r.ResourceRecordProvider.(metadata.OpenAPISpecProvider); ok {
		return provider.OpenAPISpec()
	}
	return nil
}

func (r *DefaultReconciler) ExecuteHTTPRequest(spec *managedserver.HTTPRequestSpec, payload []byte) (*managedserver.HTTPResponse, error) {
	if r == nil || r.ResourceServerManager == nil {
		return nil, errors.New("managed server is not configured")
	}
	executor, ok := r.ResourceServerManager.(managedserver.HTTPRequestExecutor)
	if !ok || executor == nil {
		return nil, errors.New("managed server must support http requests")
	}
	if spec == nil {
		return nil, errors.New("http request spec is required")
	}
	return executor.ExecuteRequest(spec, payload)
}

func (r *DefaultReconciler) RepositoryDebugInfo() (repository.RepositoryDebugInfo, bool) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return repository.RepositoryDebugInfo{}, false
	}
	if provider, ok := r.ResourceRepositoryManager.(repository.RepositoryDebugInfoProvider); ok {
		return provider.DebugInfo(), true
	}
	return repository.RepositoryDebugInfo{}, false
}

func (r *DefaultReconciler) ServerDebugInfo() (managedserver.ServerDebugInfo, bool) {
	if r == nil || r.ResourceServerManager == nil {
		return managedserver.ServerDebugInfo{}, false
	}
	if provider, ok := r.ResourceServerManager.(managedserver.ServerDebugInfoProvider); ok {
		return provider.DebugInfo(), true
	}
	return managedserver.ServerDebugInfo{}, false
}
