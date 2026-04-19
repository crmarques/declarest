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

package git

import (
	"context"
	"errors"
	"strings"

	"github.com/crmarques/declarest/faults"
	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
)

func (r *GitResourceRepository) Init(ctx context.Context) error {
	if err := r.local.Init(ctx); err != nil {
		return err
	}

	repo, err := gogit.PlainOpen(r.baseDir)
	if err != nil {
		if !errors.Is(err, gogit.ErrRepositoryNotExists) {
			return faults.Internal("failed to open repository", err)
		}
		repo, err = gogit.PlainInit(r.baseDir, false)
		if err != nil {
			return faults.Internal("failed to initialize git repository", err)
		}
	}

	if err := r.ensureRemote(repo); err != nil {
		return err
	}
	return nil
}

func (r *GitResourceRepository) Check(ctx context.Context) error {
	if err := r.local.Check(ctx); err != nil {
		if !faults.IsCategory(err, faults.NotFoundError) {
			return err
		}
	}

	_, err := r.openRepositoryForOperation(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *GitResourceRepository) ensureRemote(repo *gogit.Repository) error {
	if !r.hasRemote() {
		return nil
	}

	_, err := repo.CreateRemote(&gitcfg.RemoteConfig{
		Name: defaultRemoteName,
		URLs: []string{r.remote.URL},
	})
	if err == nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return faults.Internal("failed to configure git remote", err)
	}

	cfg, cfgErr := repo.Config()
	if cfgErr != nil {
		return faults.Internal("failed to load git config", cfgErr)
	}
	cfg.Remotes[defaultRemoteName] = &gitcfg.RemoteConfig{
		Name: defaultRemoteName,
		URLs: []string{r.remote.URL},
	}
	if setErr := repo.Storer.SetConfig(cfg); setErr != nil {
		return faults.Internal("failed to update git remote config", setErr)
	}
	return nil
}

func (r *GitResourceRepository) ensureInitializedForOperation(ctx context.Context) error {
	_, err := r.openRepositoryForOperation(ctx)
	return err
}

func (r *GitResourceRepository) openRepositoryForOperation(ctx context.Context) (*gogit.Repository, error) {
	repo, err := gogit.PlainOpen(r.baseDir)
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, gogit.ErrRepositoryNotExists) {
		return nil, faults.Internal("failed to open git repository", err)
	}

	if !r.autoInit {
		return nil, faults.NotFound("local git repository is not initialized and repository.git.local.auto-init is false", nil)
	}

	if initErr := r.Init(ctx); initErr != nil {
		return nil, initErr
	}

	repo, err = gogit.PlainOpen(r.baseDir)
	if err != nil {
		return nil, faults.Internal("failed to open git repository after initialization", err)
	}
	return repo, nil
}
