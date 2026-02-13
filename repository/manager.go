package repository

import (
	"context"

	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/resource"
)

type Manager interface {
	Save(ctx context.Context, logicalPath string, payload core.Resource) error
	Get(ctx context.Context, logicalPath string) (core.Resource, error)
	Delete(ctx context.Context, logicalPath string) error
	List(ctx context.Context, logicalPath string) ([]resource.Info, error)
	Exists(ctx context.Context, logicalPath string) (bool, error)
	Move(ctx context.Context, fromPath string, toPath string) error
	Init(ctx context.Context) error
	Refresh(ctx context.Context) error
	Reset(ctx context.Context, hard bool) error
	Check(ctx context.Context) error
	Push(ctx context.Context) error
	ForcePush(ctx context.Context) error
	PullStatus(ctx context.Context) (string, error)
}
