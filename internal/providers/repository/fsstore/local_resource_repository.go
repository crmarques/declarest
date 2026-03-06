package fsstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

var _ repository.ResourceStore = (*LocalResourceRepository)(nil)
var _ repository.RepositorySync = (*LocalResourceRepository)(nil)
var _ repository.RepositoryTreeReader = (*LocalResourceRepository)(nil)

type LocalResourceRepository struct {
	baseDir        string
	resourceFormat string
	extension      string
}

func NewLocalResourceRepository(baseDir string, resourceFormat string) *LocalResourceRepository {
	format := resourceFormat
	if format == "" {
		format = config.ResourceFormatJSON
	}

	extension := ".json"
	if format == config.ResourceFormatYAML {
		extension = ".yaml"
	}

	return &LocalResourceRepository{
		baseDir:        filepath.Clean(baseDir),
		resourceFormat: format,
		extension:      extension,
	}
}

func (r *LocalResourceRepository) Move(_ context.Context, fromPath string, toPath string) error {
	fromNormalized, err := resource.NormalizeLogicalPath(fromPath)
	if err != nil {
		return err
	}
	toNormalized, err := resource.NormalizeLogicalPath(toPath)
	if err != nil {
		return err
	}
	if fromNormalized == "/" || toNormalized == "/" {
		return faults.NewValidationError("move requires resource paths", nil)
	}

	fromFile, err := r.payloadFilePath(fromNormalized)
	if err != nil {
		return err
	}
	toFile, err := r.payloadFilePath(toNormalized)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(fromFile); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return notFoundError(fmt.Sprintf("resource %q not found", fromNormalized))
		}
		return internalError("failed to access source resource", statErr)
	}

	if err := os.MkdirAll(filepath.Dir(toFile), 0o755); err != nil {
		return internalError("failed to create destination directory", err)
	}

	if err := os.Rename(fromFile, toFile); err != nil {
		return internalError("failed to move resource", err)
	}

	_ = r.cleanupEmptyParents(filepath.Dir(fromFile))
	return nil
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
