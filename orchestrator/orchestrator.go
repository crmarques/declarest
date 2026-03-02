package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

// ServiceAccessor provides access to the individual domain services held by an
// orchestrator implementation. Clients that only need the orchestrator for
// high-level operations (read, save, mutate) should depend on the Orchestrator
// interface. Clients that need direct access to sub-services (e.g. CLI commands
// for metadata inspection or secret management) use ServiceAccessor.
type ServiceAccessor interface {
	RepositoryStore() repository.ResourceStore
	RepositorySync() repository.RepositorySync
	MetadataService() metadata.MetadataService
	SecretProvider() secrets.SecretProvider
	ManagedServerClient() managedserver.ManagedServerClient
}

type LocalReader interface {
	GetLocal(ctx context.Context, logicalPath string) (resource.Value, error)
	ListLocal(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
}

type RemoteReader interface {
	GetRemote(ctx context.Context, logicalPath string) (resource.Value, error)
	ListRemote(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
}

type OpenAPISpecReader interface {
	GetOpenAPISpec(ctx context.Context) (resource.Value, error)
}

type CompletionService interface {
	LocalReader
	RemoteReader
	OpenAPISpecReader
}

type RequestExecutor interface {
	Request(ctx context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error)
}

type RepositoryWriter interface {
	Save(ctx context.Context, logicalPath string, value resource.Value) error
}

type ResourceMutator interface {
	Apply(ctx context.Context, logicalPath string) (resource.Resource, error)
	Create(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error)
	Update(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error)
	Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error
}

type DiffReader interface {
	Explain(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
	Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error)
}

type TemplateRenderer interface {
	Template(ctx context.Context, logicalPath string, value resource.Value) (resource.Value, error)
}

type Orchestrator interface {
	LocalReader
	RemoteReader
	OpenAPISpecReader
	RequestExecutor
	RepositoryWriter
	ResourceMutator
	DiffReader
	TemplateRenderer
}
