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

package resource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestEnsureCleanGitWorktreeForAutoCommitSkipsBootstrapWhenRepoNotInitialized(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	sync := &stubRepositorySync{
		status: repository.SyncReport{
			HasUncommitted: true,
		},
	}

	err := ensureCleanGitWorktreeForAutoCommit(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local: configdomain.GitLocal{BaseDir: repoDir},
				},
			},
		},
		"resource save",
	)
	if err != nil {
		t.Fatalf("expected bootstrap save to skip clean-worktree guard, got %v", err)
	}
	if sync.syncStatusCalls != 0 {
		t.Fatalf("expected SyncStatus to be skipped for uninitialized auto-init repo, got %d calls", sync.syncStatusCalls)
	}
}

func TestEnsureCleanGitWorktreeForAutoCommitStillFailsDirtyInitializedRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	sync := &stubRepositorySync{
		status: repository.SyncReport{
			HasUncommitted: true,
		},
	}

	err := ensureCleanGitWorktreeForAutoCommit(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local: configdomain.GitLocal{BaseDir: repoDir},
				},
			},
		},
		"resource save",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if sync.syncStatusCalls != 1 {
		t.Fatalf("expected SyncStatus to run for initialized repo, got %d calls", sync.syncStatusCalls)
	}
	if sync.historyCalls != 1 {
		t.Fatalf("expected history bootstrap check for initialized dirty repo, got %d calls", sync.historyCalls)
	}
}

func TestEnsureCleanGitWorktreeForAutoCommitSkipsDirtyFreshInitializedRepoWithoutCommits(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	sync := &stubRepositorySync{
		status: repository.SyncReport{
			HasUncommitted: true,
		},
		history: []repository.HistoryEntry{},
	}

	err := ensureCleanGitWorktreeForAutoCommit(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local: configdomain.GitLocal{BaseDir: repoDir},
				},
			},
		},
		"resource save",
	)
	if err != nil {
		t.Fatalf("expected dirty fresh repo without commits to skip clean-worktree guard, got %v", err)
	}
	if sync.syncStatusCalls != 1 {
		t.Fatalf("expected SyncStatus to run for initialized repo, got %d calls", sync.syncStatusCalls)
	}
	if sync.historyCalls != 1 {
		t.Fatalf("expected history bootstrap check for initialized repo, got %d calls", sync.historyCalls)
	}
}

func TestEnsureCleanGitWorktreeForAutoCommitStillChecksWhenAutoInitDisabled(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	autoInitFalse := false
	sync := &stubRepositorySync{
		status: repository.SyncReport{
			HasUncommitted: true,
		},
	}

	err := ensureCleanGitWorktreeForAutoCommit(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local: configdomain.GitLocal{BaseDir: repoDir, AutoInit: &autoInitFalse},
				},
			},
		},
		"resource save",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if sync.syncStatusCalls != 1 {
		t.Fatalf("expected SyncStatus to run when auto-init is disabled, got %d calls", sync.syncStatusCalls)
	}
}

func TestCommitAndMaybeAutoSyncRepositoryPushesWhenAutoSyncIsUnset(t *testing.T) {
	t.Parallel()

	sync := &stubRepositorySync{}
	err := commitAndMaybeAutoSyncRepository(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local:  configdomain.GitLocal{BaseDir: t.TempDir()},
					Remote: &configdomain.GitRemote{URL: "https://example.invalid/repo.git"},
				},
			},
		},
		"test commit",
	)
	if err != nil {
		t.Fatalf("expected default auto-sync push to succeed, got %v", err)
	}
	if sync.commitCalls != 1 {
		t.Fatalf("expected one commit call, got %d", sync.commitCalls)
	}
	if sync.pushCalls != 1 {
		t.Fatalf("expected one push call when auto-sync is unset, got %d", sync.pushCalls)
	}
}

func TestCommitAndMaybeAutoSyncRepositorySkipsPushWhenAutoSyncIsFalse(t *testing.T) {
	t.Parallel()

	autoSyncFalse := false
	sync := &stubRepositorySync{}
	err := commitAndMaybeAutoSyncRepository(
		context.Background(),
		cliutil.CommandDependencies{Services: &stubLocalEditServiceAccessor{sync: sync}},
		configdomain.Context{
			Repository: configdomain.Repository{
				Git: &configdomain.GitRepository{
					Local: configdomain.GitLocal{BaseDir: t.TempDir()},
					Remote: &configdomain.GitRemote{
						URL:      "https://example.invalid/repo.git",
						AutoSync: &autoSyncFalse,
					},
				},
			},
		},
		"test commit",
	)
	if err != nil {
		t.Fatalf("expected explicit auto-sync=false to skip push cleanly, got %v", err)
	}
	if sync.commitCalls != 1 {
		t.Fatalf("expected one commit call, got %d", sync.commitCalls)
	}
	if sync.pushCalls != 0 {
		t.Fatalf("expected push to be skipped when auto-sync is false, got %d calls", sync.pushCalls)
	}
}

type stubLocalEditServiceAccessor struct {
	sync repository.RepositorySync
}

func (a *stubLocalEditServiceAccessor) RepositoryStore() repository.ResourceStore       { return nil }
func (a *stubLocalEditServiceAccessor) RepositorySync() repository.RepositorySync       { return a.sync }
func (a *stubLocalEditServiceAccessor) MetadataService() metadatadomain.MetadataService { return nil }
func (a *stubLocalEditServiceAccessor) SecretProvider() secretdomain.SecretProvider     { return nil }
func (a *stubLocalEditServiceAccessor) ManagedServiceClient() managedservicedomain.ManagedServiceClient {
	return nil
}

type stubRepositorySync struct {
	status          repository.SyncReport
	syncStatusErr   error
	syncStatusCalls int
	history         []repository.HistoryEntry
	historyErr      error
	historyCalls    int
	commitCalls     int
	pushCalls       int
}

func (s *stubRepositorySync) Init(context.Context) error                          { return nil }
func (s *stubRepositorySync) Refresh(context.Context) error                       { return nil }
func (s *stubRepositorySync) Clean(context.Context) error                         { return nil }
func (s *stubRepositorySync) Reset(context.Context, repository.ResetPolicy) error { return nil }
func (s *stubRepositorySync) Check(context.Context) error                         { return nil }
func (s *stubRepositorySync) Push(context.Context, repository.PushPolicy) error {
	s.pushCalls++
	return nil
}
func (s *stubRepositorySync) Commit(context.Context, string) (bool, error) {
	s.commitCalls++
	return true, nil
}
func (s *stubRepositorySync) SyncStatus(context.Context) (repository.SyncReport, error) {
	s.syncStatusCalls++
	if s.syncStatusErr != nil {
		return repository.SyncReport{}, s.syncStatusErr
	}
	return s.status, nil
}
func (s *stubRepositorySync) History(context.Context, repository.HistoryFilter) ([]repository.HistoryEntry, error) {
	s.historyCalls++
	if s.historyErr != nil {
		return nil, s.historyErr
	}
	if s.history == nil {
		return []repository.HistoryEntry{{Hash: "seed", Subject: "seed"}}, nil
	}
	items := make([]repository.HistoryEntry, len(s.history))
	copy(items, s.history)
	return items, nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
