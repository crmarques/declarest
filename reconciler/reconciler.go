package reconciler

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/resource"
)

type Reconciler interface {
	Get(ctx context.Context, logicalPath string) (core.Resource, error)
	Save(ctx context.Context, logicalPath string, payload core.Resource) error
	Apply(ctx context.Context, logicalPath string) (resource.Info, error)
	Create(ctx context.Context, logicalPath string, payload core.Resource) (resource.Info, error)
	Update(ctx context.Context, logicalPath string, payload core.Resource) (resource.Info, error)
	Delete(ctx context.Context, logicalPath string, recursive bool) error
	ListLocal(ctx context.Context, logicalPath string) ([]resource.Info, error)
	ListRemote(ctx context.Context, logicalPath string) ([]resource.Info, error)
	Explain(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
	Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
	Template(ctx context.Context, logicalPath string, payload core.Resource) (core.Resource, error)
	RepoInit(ctx context.Context) error
	RepoRefresh(ctx context.Context) error
	RepoPush(ctx context.Context, force bool) error
	RepoReset(ctx context.Context, hard bool) error
	RepoCheck(ctx context.Context) error
}
