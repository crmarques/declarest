package stub

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/support/notimpl"
	reconcilerdomain "github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

var _ reconcilerdomain.ResourceReconciler = (*StubResourceReconciler)(nil)

type StubResourceReconciler struct{}

func (r *StubResourceReconciler) Get(context.Context, string) (resource.Value, error) {
	return nil, notimpl.Error("StubResourceReconciler", "Get")
}

func (r *StubResourceReconciler) Save(context.Context, string, resource.Value) error {
	return notimpl.Error("StubResourceReconciler", "Save")
}

func (r *StubResourceReconciler) Apply(context.Context, string) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("StubResourceReconciler", "Apply")
}

func (r *StubResourceReconciler) Create(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("StubResourceReconciler", "Create")
}

func (r *StubResourceReconciler) Update(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, notimpl.Error("StubResourceReconciler", "Update")
}

func (r *StubResourceReconciler) Delete(context.Context, string, reconcilerdomain.DeletePolicy) error {
	return notimpl.Error("StubResourceReconciler", "Delete")
}

func (r *StubResourceReconciler) ListLocal(context.Context, string, reconcilerdomain.ListPolicy) ([]resource.Resource, error) {
	return nil, notimpl.Error("StubResourceReconciler", "ListLocal")
}

func (r *StubResourceReconciler) ListRemote(context.Context, string, reconcilerdomain.ListPolicy) ([]resource.Resource, error) {
	return nil, notimpl.Error("StubResourceReconciler", "ListRemote")
}

func (r *StubResourceReconciler) Explain(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notimpl.Error("StubResourceReconciler", "Explain")
}

func (r *StubResourceReconciler) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, notimpl.Error("StubResourceReconciler", "Diff")
}

func (r *StubResourceReconciler) Template(context.Context, string, resource.Value) (resource.Value, error) {
	return nil, notimpl.Error("StubResourceReconciler", "Template")
}

func (r *StubResourceReconciler) RepoInit(context.Context) error {
	return notimpl.Error("StubResourceReconciler", "RepoInit")
}

func (r *StubResourceReconciler) RepoRefresh(context.Context) error {
	return notimpl.Error("StubResourceReconciler", "RepoRefresh")
}

func (r *StubResourceReconciler) RepoPush(context.Context, repository.PushPolicy) error {
	return notimpl.Error("StubResourceReconciler", "RepoPush")
}

func (r *StubResourceReconciler) RepoReset(context.Context, repository.ResetPolicy) error {
	return notimpl.Error("StubResourceReconciler", "RepoReset")
}

func (r *StubResourceReconciler) RepoCheck(context.Context) error {
	return notimpl.Error("StubResourceReconciler", "RepoCheck")
}

func (r *StubResourceReconciler) RepoStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{}, notimpl.Error("StubResourceReconciler", "RepoStatus")
}
