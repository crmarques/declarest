package config

import "context"

type ContextCatalogWriter interface {
	Create(ctx context.Context, cfg Context) error
	Update(ctx context.Context, cfg Context) error
	Delete(ctx context.Context, name string) error
	Rename(ctx context.Context, fromName string, toName string) error
	SetCurrent(ctx context.Context, name string) error
}

type ContextCatalogReader interface {
	List(ctx context.Context) ([]Context, error)
	GetCurrent(ctx context.Context) (Context, error)
}

// ContextCatalogEditor is an optional capability for commands that need to
// edit the full persisted catalog while preserving strict validation.
type ContextCatalogEditor interface {
	GetCatalog(ctx context.Context) (ContextCatalog, error)
	ReplaceCatalog(ctx context.Context, catalog ContextCatalog) error
}

type ContextResolver interface {
	ResolveContext(ctx context.Context, selection ContextSelection) (Context, error)
}

type ContextValidator interface {
	Validate(ctx context.Context, cfg Context) error
}

type ContextService interface {
	ContextCatalogWriter
	ContextCatalogReader
	ContextResolver
	ContextValidator
}
