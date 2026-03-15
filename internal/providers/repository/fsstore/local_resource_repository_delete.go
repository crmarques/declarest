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
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *LocalResourceRepository) Delete(_ context.Context, logicalPath string, policy repository.DeletePolicy) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	files, err := r.discoverPayloadFiles(normalizedPath)
	if err != nil {
		return err
	}

	if files.Resource != nil || files.Defaults != nil {
		if err := r.removePayloadFile(files.Resource); err != nil {
			return err
		}
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
			files, infoErr := r.discoverPayloadFiles(logicalPath)
			if infoErr != nil {
				return infoErr
			}
			if files.Resource != nil || files.Defaults != nil {
				if err := r.removePayloadFile(files.Resource); err != nil {
					return err
				}
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
			files, infoErr := r.discoverPayloadFiles(logicalPath)
			if infoErr != nil {
				return infoErr
			}
			if files.Resource == nil && files.Defaults == nil {
				return nil
			}
			if err := r.removePayloadFile(files.Resource); err != nil {
				return err
			}
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
