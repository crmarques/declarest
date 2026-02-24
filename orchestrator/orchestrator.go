package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

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
