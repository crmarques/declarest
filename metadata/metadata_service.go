package metadata

import "context"

type MetadataStore interface {
	Get(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	Set(ctx context.Context, logicalPath string, metadata ResourceMetadata) error
	Unset(ctx context.Context, logicalPath string) error
}

type MetadataResolver interface {
	ResolveForPath(ctx context.Context, logicalPath string) (ResourceMetadata, error)
}

type OperationSpecRenderer interface {
	RenderOperationSpec(ctx context.Context, logicalPath string, operation Operation, value any) (OperationSpec, error)
}

type ResourceOperationSpecInput struct {
	LogicalPath    string
	CollectionPath string
	LocalAlias     string
	RemoteID       string
	Metadata       ResourceMetadata
	Payload        any
}

// ResourceOperationSpecRenderer is an optional metadata capability used by
// orchestrator/server adapters to render operation specs with fully derived
// resource identity/context.
type ResourceOperationSpecRenderer interface {
	RenderOperationSpecForResource(ctx context.Context, resourceInfo ResourceOperationSpecInput, operation Operation) (OperationSpec, error)
}

type MetadataService interface {
	MetadataStore
	MetadataResolver
	OperationSpecRenderer
}

// CollectionChildrenResolver is an optional metadata capability used by path
// completion to surface child selectors that exist only in metadata templates.
type CollectionChildrenResolver interface {
	ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error)
}

// CollectionWildcardResolver is an optional metadata capability used by
// fallback helpers to know when wildcard selectors are available under a
// collection branch.
type CollectionWildcardResolver interface {
	HasCollectionWildcardChild(ctx context.Context, logicalPath string) (bool, error)
}
