package fsstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/providers/fsutil"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) Delete(_ context.Context, logicalPath string, policy repository.DeletePolicy) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	info, err := r.discoverPayloadFile(normalizedPath)
	if err != nil {
		return err
	}

	if info != nil {
		if err := os.Remove(info.Path); err != nil {
			return internalError("failed to remove resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(info.Path))
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
			if entry.Name() == "_" {
				continue
			}

			resourceDir := filepath.Join(collectionPath, entry.Name())
			relativeDir, relErr := filepath.Rel(r.baseDir, resourceDir)
			if relErr != nil {
				return internalError("failed to resolve collection resource path", relErr)
			}
			logicalPath := "/" + strings.TrimPrefix(filepath.ToSlash(relativeDir), "/")
			info, infoErr := r.payloadFileInfoFromDir(logicalPath, resourceDir)
			if infoErr != nil {
				return infoErr
			}
			if info != nil {
				if err := os.Remove(info.Path); err != nil {
					return internalError("failed to delete resource from collection", err)
				}
				_ = r.cleanupEmptyParents(filepath.Dir(info.Path))
			}
			continue
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
			relativeDir, relErr := filepath.Rel(r.baseDir, filePath)
			if relErr != nil {
				return relErr
			}
			logicalPath := "/" + strings.TrimPrefix(filepath.ToSlash(relativeDir), "/")
			info, infoErr := r.payloadFileInfoFromDir(logicalPath, filePath)
			if infoErr != nil {
				return infoErr
			}
			if info == nil {
				return nil
			}
			if err := os.Remove(info.Path); err != nil {
				return err
			}
			_ = r.cleanupEmptyParents(filepath.Dir(info.Path))
			return nil
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
