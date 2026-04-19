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
		return faults.Invalid("repository base directory must not be empty", nil)
	}
	if err := os.MkdirAll(r.baseDir, 0o755); err != nil {
		return faults.Internal("failed to initialize repository directory", err)
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
			return faults.NotFound("repository base directory does not exist", nil)
		}
		return faults.Internal("failed to inspect repository base directory", err)
	}
	if !info.IsDir() {
		return faults.Invalid("repository base directory is not a directory", nil)
	}
	return nil
}

func (r *LocalResourceRepository) Push(context.Context, repository.PushPolicy) error {
	return faults.Invalid("push requires git repository with remote configuration", nil)
}

func (r *LocalResourceRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
}
