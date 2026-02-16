package localfs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/support/fsutil"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) Delete(_ context.Context, logicalPath string, policy repository.DeletePolicy) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return err
	}

	if stat, statErr := os.Stat(payloadPath); statErr == nil && !stat.IsDir() {
		if err := os.Remove(payloadPath); err != nil {
			return internalError("failed to remove resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(payloadPath))
		return nil
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return err
	}

	if policy.Recursive {
		return r.deleteCollectionRecursive(collectionPath)
	}
	return r.deleteCollectionDirect(collectionPath)
}

func (r *LocalResourceRepository) deleteCollectionDirect(collectionPath string) error {
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to list collection for delete", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			continue
		}
		target := filepath.Join(collectionPath, entry.Name())
		if err := os.Remove(target); err != nil {
			return internalError("failed to delete resource from collection", err)
		}
	}
	return nil
}

func (r *LocalResourceRepository) deleteCollectionRecursive(collectionPath string) error {
	err := filepath.WalkDir(collectionPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "_" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			return nil
		}
		if err := os.Remove(filePath); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return internalError("failed to recursively delete collection resources", err)
	}
	return nil
}

func (r *LocalResourceRepository) cleanupEmptyParents(startDir string) error {
	return fsutil.CleanupEmptyParents(startDir, r.baseDir)
}
