package defaultreconciler

import (
	"context"
	"sync"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	reconcilerdomain "github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

var _ reconcilerdomain.ResourceReconciler = (*DefaultResourceReconciler)(nil)

type RepositoryResolver func(ctx context.Context) (repository.ResourceRepository, error)

type DefaultResourceReconciler struct {
	resolveRepository RepositoryResolver
	repoOnce          sync.Once
	repo              repository.ResourceRepository
	repoErr           error
}

func NewResourceReconciler(resolveRepository RepositoryResolver) *DefaultResourceReconciler {
	return &DefaultResourceReconciler{
		resolveRepository: resolveRepository,
	}
}

func (r *DefaultResourceReconciler) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	repo, err := r.repository(ctx)
	if err != nil {
		return nil, err
	}
	return repo.Get(ctx, logicalPath)
}

func (r *DefaultResourceReconciler) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Save(ctx, logicalPath, value)
}

func (r *DefaultResourceReconciler) Apply(context.Context, string) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("DefaultResourceReconciler", "Apply")
}

func (r *DefaultResourceReconciler) Create(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("DefaultResourceReconciler", "Create")
}

func (r *DefaultResourceReconciler) Update(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("DefaultResourceReconciler", "Update")
}

func (r *DefaultResourceReconciler) Delete(ctx context.Context, logicalPath string, policy reconcilerdomain.DeletePolicy) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Delete(ctx, logicalPath, repository.DeletePolicy{Recursive: policy.Recursive})
}

func (r *DefaultResourceReconciler) ListLocal(ctx context.Context, logicalPath string, policy reconcilerdomain.ListPolicy) ([]resource.Resource, error) {
	repo, err := r.repository(ctx)
	if err != nil {
		return nil, err
	}
	return repo.List(ctx, logicalPath, repository.ListPolicy{Recursive: policy.Recursive})
}

func (r *DefaultResourceReconciler) ListRemote(context.Context, string, reconcilerdomain.ListPolicy) ([]resource.Resource, error) {
	return nil, notimpl.Error("DefaultResourceReconciler", "ListRemote")
}

func (r *DefaultResourceReconciler) Explain(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notimpl.Error("DefaultResourceReconciler", "Explain")
}

func (r *DefaultResourceReconciler) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notimpl.Error("DefaultResourceReconciler", "Diff")
}

func (r *DefaultResourceReconciler) Template(context.Context, string, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("DefaultResourceReconciler", "Template")
}

func (r *DefaultResourceReconciler) RepoInit(ctx context.Context) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Init(ctx)
}

func (r *DefaultResourceReconciler) RepoRefresh(ctx context.Context) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Refresh(ctx)
}

func (r *DefaultResourceReconciler) RepoPush(ctx context.Context, policy repository.PushPolicy) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Push(ctx, policy)
}

func (r *DefaultResourceReconciler) RepoReset(ctx context.Context, policy repository.ResetPolicy) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Reset(ctx, policy)
}

func (r *DefaultResourceReconciler) RepoCheck(ctx context.Context) error {
	repo, err := r.repository(ctx)
	if err != nil {
		return err
	}
	return repo.Check(ctx)
}

func (r *DefaultResourceReconciler) RepoStatus(ctx context.Context) (repository.SyncReport, error) {
	repo, err := r.repository(ctx)
	if err != nil {
		return repository.SyncReport{}, err
	}
	return repo.SyncStatus(ctx)
}

func (r *DefaultResourceReconciler) repository(ctx context.Context) (repository.ResourceRepository, error) {
	if r.resolveRepository == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository resolver is not configured", nil)
	}

	r.repoOnce.Do(func() {
		r.repo, r.repoErr = r.resolveRepository(ctx)
	})

	if r.repoErr != nil {
		return nil, r.repoErr
	}
	if r.repo == nil {
		return nil, faults.NewTypedError(faults.InternalError, "repository resolver returned nil repository", nil)
	}
	return r.repo, nil
}
