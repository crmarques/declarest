package managedserver

import (
	"context"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type RequestSpec struct {
	Method      string
	Path        string
	Query       map[string]string
	Headers     map[string]string
	Accept      string
	ContentType string
	Body        resource.Content
}

type ManagedServerClient interface {
	Get(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Create(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Update(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Delete(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) error
	List(ctx context.Context, collectionPath string, metadata metadata.ResourceMetadata) ([]resource.Resource, error)
	Exists(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (bool, error)
	Request(ctx context.Context, spec RequestSpec) (resource.Content, error)
	GetOpenAPISpec(ctx context.Context) (resource.Content, error)
}

// AccessTokenProvider is an optional managed-server capability used by CLI
// inspection commands to retrieve an OAuth2 access token when supported.
type AccessTokenProvider interface {
	GetAccessToken(ctx context.Context) (string, error)
}
