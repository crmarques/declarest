// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fsstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) List(_ context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	files, err := r.discoverPayloadFiles(normalizedPath)
	if err != nil {
		return nil, err
	}
	if primary := files.primary(); primary != nil {
		return []resource.Resource{buildListedResource(normalizedPath, primary)}, nil
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

	files, err := r.discoverPayloadFiles(normalizedPath)
	if err != nil {
		return false, err
	}
	if files.primary() != nil {
		return true, nil
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
		return false, faults.Internal("failed to check collection path", err)
	}
}

func (r *LocalResourceRepository) listDirect(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	entries, err := os.ReadDir(collectionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, faults.Internal("failed to list collection", err)
	}

	itemsByPath := make(map[string]resource.Resource)
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "_" {
				continue
			}

			logicalPath := path.Join(baseLogicalPath, entry.Name())
			if !strings.HasPrefix(logicalPath, "/") {
				logicalPath = "/" + logicalPath
			}

			files, infoErr := r.discoverPayloadFiles(logicalPath)
			if infoErr != nil {
				return nil, infoErr
			}
			if primary := files.primary(); primary != nil {
				itemsByPath[logicalPath] = buildListedResource(logicalPath, primary)
			}
			continue
		}
	}

	items := make([]resource.Resource, 0, len(itemsByPath))
	for _, item := range itemsByPath {
		items = append(items, item)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func (r *LocalResourceRepository) listRecursive(baseLogicalPath string, collectionPath string) ([]resource.Resource, error) {
	itemsByPath := make(map[string]resource.Resource)

	err := filepath.WalkDir(collectionPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == "_" {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(collectionPath, filePath)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)

		if hasReservedSegment(relPath) {
			return nil
		}
		logicalPath := baseLogicalPath
		if relPath != "." {
			logicalPath = path.Join(baseLogicalPath, relPath)
		}
		if !strings.HasPrefix(logicalPath, "/") {
			logicalPath = "/" + logicalPath
		}
		files, infoErr := r.discoverPayloadFiles(logicalPath)
		if infoErr != nil {
			return infoErr
		}
		if primary := files.primary(); primary != nil {
			itemsByPath[logicalPath] = buildListedResource(logicalPath, primary)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, faults.Internal("failed to walk collection", err)
	}

	items := make([]resource.Resource, 0, len(itemsByPath))
	for _, item := range itemsByPath {
		items = append(items, item)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func buildListedResource(logicalPath string, info *payloadFileInfo) resource.Resource {
	collectionPath := path.Dir(logicalPath)
	if collectionPath == "." {
		collectionPath = "/"
	}
	if collectionPath == "" {
		collectionPath = "/"
	}
	descriptor := resource.PayloadDescriptor{}
	if info != nil {
		descriptor = info.Descriptor
	}
	return resource.Resource{
		LogicalPath:       logicalPath,
		CollectionPath:    collectionPath,
		LocalAlias:        path.Base(logicalPath),
		PayloadDescriptor: descriptor,
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
