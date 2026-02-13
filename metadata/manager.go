package metadata

import (
	"context"

	"github.com/crmarques/declarest/core"
)

type Manager interface {
	Get(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	Set(ctx context.Context, logicalPath string, metadata ResourceMetadata) error
	Unset(ctx context.Context, logicalPath string) error
	ResolveForPath(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	RenderOperationSpec(ctx context.Context, logicalPath string, operation string, payload core.Resource) (OperationSpec, error)
	Infer(ctx context.Context, logicalPath string, apply bool, recursive bool) (ResourceMetadata, error)
}
