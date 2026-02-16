package fs

import (
	"context"

	"github.com/crmarques/declarest/internal/providers/repository/localfs"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

var _ repository.ResourceRepository = (*FSResourceRepository)(nil)

type FSResourceRepository struct {
	local *localfs.LocalResourceRepository
}

func NewFSResourceRepository(baseDir string, resourceFormat string) *FSResourceRepository {
	return &FSResourceRepository{
		local: localfs.NewLocalResourceRepository(baseDir, resourceFormat),
	}
}

func (r *FSResourceRepository) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	return r.local.Save(ctx, logicalPath, value)
}

func (r *FSResourceRepository) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	return r.local.Get(ctx, logicalPath)
}

func (r *FSResourceRepository) Delete(ctx context.Context, logicalPath string, policy repository.DeletePolicy) error {
	return r.local.Delete(ctx, logicalPath, policy)
}

func (r *FSResourceRepository) List(ctx context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	return r.local.List(ctx, logicalPath, policy)
}

func (r *FSResourceRepository) Exists(ctx context.Context, logicalPath string) (bool, error) {
	return r.local.Exists(ctx, logicalPath)
}

func (r *FSResourceRepository) Move(ctx context.Context, fromPath string, toPath string) error {
	return r.local.Move(ctx, fromPath, toPath)
}

func (r *FSResourceRepository) Init(ctx context.Context) error {
	return r.local.Init(ctx)
}

func (r *FSResourceRepository) Refresh(ctx context.Context) error {
	return r.local.Refresh(ctx)
}

func (r *FSResourceRepository) Reset(ctx context.Context, policy repository.ResetPolicy) error {
	return r.local.Reset(ctx, policy)
}

func (r *FSResourceRepository) Check(ctx context.Context) error {
	return r.local.Check(ctx)
}

func (r *FSResourceRepository) Push(ctx context.Context, policy repository.PushPolicy) error {
	return r.local.Push(ctx, policy)
}

func (r *FSResourceRepository) SyncStatus(ctx context.Context) (repository.SyncReport, error) {
	return r.local.SyncStatus(ctx)
}
