package secrets

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

type SecretStore interface {
	Init(ctx context.Context) error
	Store(ctx context.Context, key string, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
}

type PayloadSecretProcessor interface {
	MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error)
	ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error)
	NormalizeSecretPlaceholders(ctx context.Context, value resource.Value) (resource.Value, error)
}

type SecretCandidateDetector interface {
	DetectSecretCandidates(ctx context.Context, value resource.Value) ([]string, error)
}

type SecretProvider interface {
	SecretStore
	PayloadSecretProcessor
	SecretCandidateDetector
}
