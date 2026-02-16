package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

type Orchestrator interface {
	Get(ctx context.Context, logicalPath string) (resource.Value, error)
	GetLocal(ctx context.Context, logicalPath string) (resource.Value, error)
	GetRemote(ctx context.Context, logicalPath string) (resource.Value, error)
	Save(ctx context.Context, logicalPath string, value resource.Value) error
	Apply(ctx context.Context, logicalPath string) (resource.Resource, error)
	Create(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error)
	Update(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error)
	Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error
	ListLocal(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
	ListRemote(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
	Explain(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
	Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
	Template(ctx context.Context, logicalPath string, value resource.Value) (resource.Value, error)
}
