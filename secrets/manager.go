package secrets

import (
	"context"

	"github.com/crmarques/declarest/core"
)

type Manager interface {
	Init(ctx context.Context) error
	Store(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
	MaskPayload(ctx context.Context, payload core.Resource) (core.Resource, error)
	ResolvePayload(ctx context.Context, payload core.Resource) (core.Resource, error)
	NormalizeSecretPlaceholders(ctx context.Context, payload core.Resource) (core.Resource, error)
	DetectSecretCandidates(ctx context.Context, payload core.Resource) ([]string, error)
}
