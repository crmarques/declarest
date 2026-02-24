package fsstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/providers/shared/fsutil"
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

		legacyPayloadPath, legacyErr := r.legacyPayloadFilePath(normalizedPath)
		if legacyErr == nil {
			if _, legacyStatErr := os.Stat(legacyPayloadPath); legacyStatErr == nil {
				if removeErr := os.Remove(legacyPayloadPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					return internalError("failed to remove legacy resource payload", removeErr)
				}
				_ = r.cleanupEmptyParents(filepath.Dir(legacyPayloadPath))
			} else if legacyStatErr != nil && !errors.Is(legacyStatErr, os.ErrNotExist) {
				return internalError("failed to inspect legacy resource payload", legacyStatErr)
			}
		}
		return nil
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return internalError("failed to inspect resource payload", statErr)
	}

	legacyPayloadPath, err := r.legacyPayloadFilePath(normalizedPath)
	if err != nil {
		return err
	}
	if stat, statErr := os.Stat(legacyPayloadPath); statErr == nil && !stat.IsDir() {
		if err := os.Remove(legacyPayloadPath); err != nil {
			return internalError("failed to remove resource payload", err)
		}
		_ = r.cleanupEmptyParents(filepath.Dir(legacyPayloadPath))
		return nil
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return internalError("failed to inspect resource payload", statErr)
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

			resourceFilePath := filepath.Join(collectionPath, entry.Name(), r.resourceFileName())
			if stat, statErr := os.Stat(resourceFilePath); statErr == nil && !stat.IsDir() {
				if err := os.Remove(resourceFilePath); err != nil {
					return internalError("failed to delete resource from collection", err)
				}
				_ = r.cleanupEmptyParents(filepath.Dir(resourceFilePath))
			} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return internalError("failed to inspect collection resource payload", statErr)
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			continue
		}
		if entry.Name() == r.resourceFileName() || entry.Name() == r.metadataFileName() {
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
		_ = r.cleanupEmptyParents(filepath.Dir(filePath))
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
