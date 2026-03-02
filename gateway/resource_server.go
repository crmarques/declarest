package gateway

import (
	"context"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type ResourceGateway interface {
	Get(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error)
	Create(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error)
	Update(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error)
	Delete(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) error
	List(ctx context.Context, collectionPath string, metadata metadata.ResourceMetadata) ([]resource.Resource, error)
	Exists(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (bool, error)
	Request(ctx context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error)
	GetOpenAPISpec(ctx context.Context) (resource.Value, error)
}

// AccessTokenProvider is an optional resource-server capability used by CLI
// inspection commands to retrieve an OAuth2 access token when supported.
type AccessTokenProvider interface {
	GetAccessToken(ctx context.Context) (string, error)
}
