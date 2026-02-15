package config

import "context"

type ContextService interface {
	Create(ctx context.Context, cfg Context) error
	Update(ctx context.Context, cfg Context) error
	Delete(ctx context.Context, name string) error
	Rename(ctx context.Context, fromName string, toName string) error
	List(ctx context.Context) ([]Context, error)
	SetCurrent(ctx context.Context, name string) error
	GetCurrent(ctx context.Context) (Context, error)
	ResolveContext(ctx context.Context, selection ContextSelection) (Context, error)
	Validate(ctx context.Context, cfg Context) error
}
