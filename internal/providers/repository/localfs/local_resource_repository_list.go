package localfs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) List(_ context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return nil, err
	}
	if stat, statErr := os.Stat(payloadPath); statErr == nil && !stat.IsDir() {
		return []resource.Resource{buildListedResource(normalizedPath)}, nil
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return nil, err
	}

	if policy.Recursive {
		return r.listRecursive(normalizedPath, collectionPath)
	}
	return r.listDirect(normalizedPath, collectionPath)
}

func (r *LocalResourceRepository) Exists(_ context.Context, logicalPath string) (bool, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return false, err
	}

	payloadPath, err := r.payloadFilePath(normalizedPath)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(payloadPath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, internalError("failed to check resource payload", err)
	}

	collectionPath, err := r.collectionDirPath(normalizedPath)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(collectionPath); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, internalError("failed to check collection path", err)
	}
}

func (r *LocalResourceRepository) listDirect(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, internalError("failed to list collection", err)
	}

	items := make([]resource.Resource, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), r.extension) {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), r.extension)
		logicalPath := path.Join(baseLogicalPath, name)
		if !strings.HasPrefix(logicalPath, "/") {
			logicalPath = "/" + logicalPath
		}
		items = append(items, buildListedResource(logicalPath))
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func (r *LocalResourceRepository) listRecursive(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0)

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

		relPath, relErr := filepath.Rel(collectionPath, filePath)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)
		noExt := strings.TrimSuffix(relPath, r.extension)
		if hasReservedSegment(noExt) {
			return nil
		}

		logicalPath := path.Join(baseLogicalPath, noExt)
		if !strings.HasPrefix(logicalPath, "/") {
			logicalPath = "/" + logicalPath
		}
		items = append(items, buildListedResource(logicalPath))
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, internalError("failed to walk collection", err)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func buildListedResource(logicalPath string) resource.Resource {
	collectionPath := path.Dir(logicalPath)
	if collectionPath == "." {
		collectionPath = "/"
	}
	if collectionPath == "" {
		collectionPath = "/"
	}
	return resource.Resource{
		LogicalPath:    logicalPath,
		CollectionPath: collectionPath,
		LocalAlias:     path.Base(logicalPath),
	}
}

func hasReservedSegment(value string) bool {
	segments := strings.Split(value, "/")
	for _, segment := range segments {
		if segment == "_" {
			return true
		}
	}
	return false
}
