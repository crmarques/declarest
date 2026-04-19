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
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
)

func (r *LocalResourceRepository) Tree(ctx context.Context) ([]string, error) {
	if err := r.Check(ctx); err != nil {
		return nil, err
	}

	root := filepath.Clean(strings.TrimSpace(r.baseDir))
	if root == "" || root == "." {
		return nil, faults.Invalid("repository base directory is not configured", nil)
	}

	paths := make([]string, 0, 32)
	walkErr := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if current == root {
			return nil
		}

		name := entry.Name()
		if name == "_" || strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(relPath))
		return nil
	})
	if walkErr != nil {
		return nil, faults.Internal("failed to walk repository directory tree", walkErr)
	}

	sort.Strings(paths)
	return paths, nil
}
