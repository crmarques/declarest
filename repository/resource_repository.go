package repository

import (
	"context"

	"github.com/crmarques/declarest/resource"
)

// ResourceStore manages deterministic local resource persistence operations.
type ResourceStore interface {
	Save(ctx context.Context, logicalPath string, value resource.Value) error
	Get(ctx context.Context, logicalPath string) (resource.Value, error)
	Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error
	List(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error)
	Exists(ctx context.Context, logicalPath string) (bool, error)
}

// RepositorySync manages repository lifecycle and synchronization operations.
type RepositorySync interface {
	Init(ctx context.Context) error
	Refresh(ctx context.Context) error
	Reset(ctx context.Context, policy ResetPolicy) error
	Check(ctx context.Context) error
	Push(ctx context.Context, policy PushPolicy) error
	SyncStatus(ctx context.Context) (SyncReport, error)
}
