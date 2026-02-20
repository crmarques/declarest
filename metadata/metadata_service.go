package metadata

import (
	"context"
)

type MetadataService interface {
	Get(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	Set(ctx context.Context, logicalPath string, metadata ResourceMetadata) error
	Unset(ctx context.Context, logicalPath string) error
	ResolveForPath(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	RenderOperationSpec(ctx context.Context, logicalPath string, operation Operation, value any) (OperationSpec, error)
}
