package fsstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
)

var _ repository.ResourceStore = (*LocalResourceRepository)(nil)
var _ repository.RepositorySync = (*LocalResourceRepository)(nil)
var _ repository.RepositoryTreeReader = (*LocalResourceRepository)(nil)
var _ repository.ResourceArtifactStore = (*LocalResourceRepository)(nil)

type LocalResourceRepository struct {
	baseDir         string
	metadataBaseDir string
}

func NewLocalResourceRepository(baseDir string, metadataBaseDir ...string) *LocalResourceRepository {
	return &LocalResourceRepository{
		baseDir:         filepath.Clean(baseDir),
		metadataBaseDir: firstMetadataBaseDir(metadataBaseDir),
	}
}

func (r *LocalResourceRepository) Init(_ context.Context) error {
	if r.baseDir == "" {
		return faults.NewValidationError("repository base directory must not be empty", nil)
	}
	if err := os.MkdirAll(r.baseDir, 0o755); err != nil {
		return internalError("failed to initialize repository directory", err)
	}
	return nil
}

func (r *LocalResourceRepository) Refresh(context.Context) error {
	return nil
}

func (r *LocalResourceRepository) Clean(context.Context) error {
	return nil
}

func (r *LocalResourceRepository) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}

func (r *LocalResourceRepository) Check(_ context.Context) error {
	info, err := os.Stat(r.baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return notFoundError("repository base directory does not exist")
		}
		return internalError("failed to inspect repository base directory", err)
	}
	if !info.IsDir() {
		return faults.NewValidationError("repository base directory is not a directory", nil)
	}
	return nil
}

func (r *LocalResourceRepository) Push(context.Context, repository.PushPolicy) error {
	return faults.NewValidationError("push requires git repository with remote configuration", nil)
}

func (r *LocalResourceRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
