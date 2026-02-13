package ctx

import "context"

type Manager interface {
	Create(ctx context.Context, cfg Config) error
	Update(ctx context.Context, cfg Config) error
	Delete(ctx context.Context, name string) error
	Rename(ctx context.Context, fromName string, toName string) error
	List(ctx context.Context) ([]Config, error)
	SetCurrent(ctx context.Context, name string) error
	GetCurrent(ctx context.Context) (Config, error)
	LoadResolvedConfig(ctx context.Context, name string, overrides map[string]string) (Runtime, error)
	Validate(ctx context.Context, cfg Config) error
}
