package secrets

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

type SecretProvider interface {
	Init(ctx context.Context) error
	Store(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
	MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error)
	ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error)
	NormalizeSecretPlaceholders(ctx context.Context, value resource.Value) (resource.Value, error)
	DetectSecretCandidates(ctx context.Context, value resource.Value) ([]string, error)
}
