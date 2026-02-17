package server

import (
	"context"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type ResourceServer interface {
	Get(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error)
	Create(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error)
	Update(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error)
	Delete(ctx context.Context, resourceInfo resource.Resource) error
	List(ctx context.Context, collectionPath string, metadata metadata.ResourceMetadata) ([]resource.Resource, error)
	Exists(ctx context.Context, resourceInfo resource.Resource) (bool, error)
	AdHoc(ctx context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error)
	GetOpenAPISpec(ctx context.Context) (resource.Value, error)
	BuildRequestFromMetadata(ctx context.Context, resourceInfo resource.Resource, operation metadata.Operation) (metadata.OperationSpec, error)
}
