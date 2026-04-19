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
	"errors"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	"github.com/crmarques/declarest/internal/providers/repository/fsstore"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
	"github.com/crmarques/declarest/repository"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var _ repository.ResourceStore = (*GitResourceRepository)(nil)

var _ repository.RepositorySync = (*GitResourceRepository)(nil)

var _ repository.ResourceArtifactStore = (*GitResourceRepository)(nil)

var _ repository.RepositoryCommitter = (*GitResourceRepository)(nil)

var _ repository.RepositoryHistoryReader = (*GitResourceRepository)(nil)

var _ repository.RepositoryTreeReader = (*GitResourceRepository)(nil)

var _ repository.RepositoryStatusDetailsReader = (*GitResourceRepository)(nil)

const (
	defaultRemoteName = "origin"
	defaultBranchName = "main"
)

type GitResourceRepository struct {
	local    *fsstore.LocalResourceRepository
	baseDir  string
	remote   *config.GitRemote
	proxy    *config.HTTPProxy
	autoInit bool
	runtime  *promptauth.Runtime
}

type Option func(*GitResourceRepository)

func WithPromptRuntime(runtime *promptauth.Runtime) Option {
	return func(repository *GitResourceRepository) {
		if repository == nil {
			return
		}
		repository.runtime = runtime
	}
}

func NewGitResourceRepository(repoConfig config.GitRepository, opts ...Option) *GitResourceRepository {
	var remoteProxy *config.HTTPProxy
	if repoConfig.Remote != nil {
		remoteProxy = proxyhelper.Clone(repoConfig.Remote.Proxy)
	}
	repository := &GitResourceRepository{
		local:    fsstore.NewLocalResourceRepository(repoConfig.Local.BaseDir),
		baseDir:  repoConfig.Local.BaseDir,
		remote:   repoConfig.Remote,
		proxy:    remoteProxy,
		autoInit: repoConfig.Local.AutoInitEnabled(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(repository)
	}
	return repository
}

func (r *GitResourceRepository) hasRemote() bool {
	return r.remote != nil && strings.TrimSpace(r.remote.URL) != ""
}

func (r *GitResourceRepository) targetBranch() string {
	if r.remote != nil && strings.TrimSpace(r.remote.Branch) != "" {
		return strings.TrimSpace(r.remote.Branch)
	}
	return defaultBranchName
}

func classifyRemoteError(message string, err error) error {
	lower := strings.ToLower(err.Error())

	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired) ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "permission denied"):
		return faults.Auth(message, err)
	case strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "fetch first") ||
		strings.Contains(lower, "rejected"):
		return faults.Conflict(message, err)
	case strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "tls") ||
		strings.Contains(lower, "connection") ||
		strings.Contains(lower, "network"):
		return faults.Transport(message, err)
	default:
		return faults.Internal(message, err)
	}
}
