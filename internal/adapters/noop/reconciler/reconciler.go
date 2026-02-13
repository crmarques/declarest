package noop

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"
)

var _ reconciler.Reconciler = (*Reconciler)(nil)

type Reconciler struct{}

func (r *Reconciler) Get(context.Context, string) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) Save(context.Context, string, core.Resource) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) Apply(context.Context, string) (resource.Info, error) {
	return resource.Info{}, core.ErrToBeImplemented
}

func (r *Reconciler) Create(context.Context, string, core.Resource) (resource.Info, error) {
	return resource.Info{}, core.ErrToBeImplemented
}

func (r *Reconciler) Update(context.Context, string, core.Resource) (resource.Info, error) {
	return resource.Info{}, core.ErrToBeImplemented
}

func (r *Reconciler) Delete(context.Context, string, bool) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) ListLocal(context.Context, string) ([]resource.Info, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) ListRemote(context.Context, string) ([]resource.Info, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) Explain(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) Template(context.Context, string, core.Resource) (core.Resource, error) {
	return nil, core.ErrToBeImplemented
}

func (r *Reconciler) RepoInit(context.Context) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) RepoRefresh(context.Context) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) RepoPush(context.Context, bool) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) RepoReset(context.Context, bool) error {
	return core.ErrToBeImplemented
}

func (r *Reconciler) RepoCheck(context.Context) error {
	return core.ErrToBeImplemented
}
