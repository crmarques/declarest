package server

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type Manager interface {
	Get(ctx context.Context, resourceInfo resource.Info) (core.Resource, error)
	Create(ctx context.Context, resourceInfo resource.Info) (core.Resource, error)
	Update(ctx context.Context, resourceInfo resource.Info) (core.Resource, error)
	Delete(ctx context.Context, resourceInfo resource.Info) error
	List(ctx context.Context, collectionPath string, metadata metadata.ResourceMetadata) ([]resource.Info, error)
	Exists(ctx context.Context, resourceInfo resource.Info) (bool, error)
	GetOpenAPISpec(ctx context.Context) (core.Resource, error)
	BuildRequestFromMetadata(ctx context.Context, resourceInfo resource.Info, operation string) (metadata.OperationSpec, error)
}
