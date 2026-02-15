package reconciler

import (
	"context"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

var _ ResourceReconciler = (*DefaultReconciler)(nil)

type DefaultReconciler struct {
	Name              string
	RepositoryManager repository.ResourceRepositoryManager
	MetadataService   metadata.MetadataService
	ServerManager     server.ResourceServerManager
	SecretsProvider   secrets.SecretProvider
}

func (r *DefaultReconciler) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return nil, err
	}
	return manager.Get(ctx, logicalPath)
}

func (r *DefaultReconciler) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Save(ctx, logicalPath, value)
}

func (r *DefaultReconciler) Apply(context.Context, string) (resource.Resource, error) {
	return resource.Resource{}, notImplementedError("Apply")
}

func (r *DefaultReconciler) Create(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notImplementedError("Create")
}

func (r *DefaultReconciler) Update(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notImplementedError("Update")
}

func (r *DefaultReconciler) Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Delete(ctx, logicalPath, repository.DeletePolicy{Recursive: policy.Recursive})
}

func (r *DefaultReconciler) ListLocal(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return nil, err
	}
	return manager.List(ctx, logicalPath, repository.ListPolicy{Recursive: policy.Recursive})
}

func (r *DefaultReconciler) ListRemote(context.Context, string, ListPolicy) ([]resource.Resource, error) {
	return nil, notImplementedError("ListRemote")
}

func (r *DefaultReconciler) Explain(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notImplementedError("Explain")
}

func (r *DefaultReconciler) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notImplementedError("Diff")
}

func (r *DefaultReconciler) Template(context.Context, string, resource.Value) (resource.Value, error) {
	return nil, notImplementedError("Template")
}

func (r *DefaultReconciler) RepoInit(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Init(ctx)
}

func (r *DefaultReconciler) RepoRefresh(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Refresh(ctx)
}

func (r *DefaultReconciler) RepoPush(ctx context.Context, policy repository.PushPolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Push(ctx, policy)
}

func (r *DefaultReconciler) RepoReset(ctx context.Context, policy repository.ResetPolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Reset(ctx, policy)
}

func (r *DefaultReconciler) RepoCheck(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Check(ctx)
}

func (r *DefaultReconciler) RepoStatus(ctx context.Context) (repository.SyncReport, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return repository.SyncReport{}, err
	}
	return manager.SyncStatus(ctx)
}

func (r *DefaultReconciler) repositoryManager() (repository.ResourceRepositoryManager, error) {
	if r == nil || r.RepositoryManager == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository manager is not configured", nil)
	}
	return r.RepositoryManager, nil
}

func notImplementedError(method string) error {
	return faults.NewTypedError(
		faults.InternalError,
		"DefaultReconciler."+method+" not implemented",
		faults.ErrToBeImplemented,
	)
}
