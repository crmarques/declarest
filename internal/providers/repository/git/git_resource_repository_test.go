package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	gogit "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGitRepositoryNoRemoteStatus(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)
	if err := provider.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	status, err := provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus returned error: %v", err)
	}
	if status.State != repository.SyncStateNoRemote {
		t.Fatalf("expected no_remote, got %q", status.State)
	}
}

func TestGitRepositoryAutoInitOnSyncStatus(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)

	status, err := provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus returned error: %v", err)
	}
	if status.State != repository.SyncStateNoRemote {
		t.Fatalf("expected no_remote after auto-init, got %q", status.State)
	}

	assertGitRepoInitialized(t, repoDir)
}

func TestGitRepositoryAutoInitOnHistoryReturnsEmpty(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)

	history, err := provider.History(context.Background(), repository.HistoryFilter{})
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history for fresh auto-initialized repo, got %#v", history)
	}

	assertGitRepoInitialized(t, repoDir)
}

func TestGitRepositoryAutoInitOnCommitWithoutChanges(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)

	committed, err := provider.Commit(context.Background(), "test commit")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}
	if committed {
		t.Fatal("expected committed=false for fresh auto-initialized repo without changes")
	}

	assertGitRepoInitialized(t, repoDir)
}

func TestGitRepositoryAutoInitOnCheck(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)

	if err := provider.Check(context.Background()); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	assertGitRepoInitialized(t, repoDir)
}

func TestGitRepositoryAutoInitDisabledSyncStatusFailsWithoutInitialization(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: repoDir, AutoInit: boolPtr(false)},
		},
		config.ResourceFormatJSON,
	)

	_, err := provider.SyncStatus(context.Background())
	assertCategory(t, err, faults.NotFoundError)
	assertGitRepoNotInitialized(t, repoDir)
}

func TestGitRepositoryAutoInitDisabledResourceOpsFailWithoutInitialization(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: repoDir, AutoInit: boolPtr(false)},
		},
		config.ResourceFormatJSON,
	)

	_, err := provider.Exists(context.Background(), "/customers/acme")
	assertCategory(t, err, faults.NotFoundError)
	assertGitRepoNotInitialized(t, repoDir)
}

func TestGitRepositoryExplicitInitStillWorksWhenAutoInitDisabled(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: repoDir, AutoInit: boolPtr(false)},
		},
		config.ResourceFormatJSON,
	)

	if err := provider.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	assertGitRepoInitialized(t, repoDir)
}

func TestGitRepositoryPushWithoutRemote(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)
	if err := provider.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	err := provider.Push(context.Background(), repository.PushPolicy{})
	assertCategory(t, err, faults.ValidationError)
}

func TestGitRepositoryCleanRemovesUncommittedTrackedAndUntrackedChanges(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	provider := NewGitResourceRepository(
		config.GitRepository{Local: config.GitLocal{BaseDir: repoDir}},
		config.ResourceFormatJSON,
	)
	if err := provider.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	commitFile(t, repo, repoDir, "tracked.txt", "v1", "seed tracked file")

	trackedPath := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(trackedPath, []byte("v2"), 0o600); err != nil {
		t.Fatalf("failed to modify tracked file: %v", err)
	}
	untrackedPath := filepath.Join(repoDir, "tmp", "scratch.txt")
	if err := os.MkdirAll(filepath.Dir(untrackedPath), 0o755); err != nil {
		t.Fatalf("failed to create untracked dir: %v", err)
	}
	if err := os.WriteFile(untrackedPath, []byte("scratch"), 0o600); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	statusBefore, err := provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus before clean returned error: %v", err)
	}
	if !statusBefore.HasUncommitted {
		t.Fatal("expected hasUncommitted=true before clean")
	}

	if err := provider.Clean(context.Background()); err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	statusAfter, err := provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus after clean returned error: %v", err)
	}
	if statusAfter.HasUncommitted {
		t.Fatal("expected hasUncommitted=false after clean")
	}

	trackedContent, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("failed to read tracked file after clean: %v", err)
	}
	if string(trackedContent) != "v1" {
		t.Fatalf("expected tracked file to be reset to committed content, got %q", string(trackedContent))
	}
	if _, err := os.Stat(untrackedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected untracked file to be removed, got err=%v", err)
	}
}

func TestGitRepositoryTargetBranchDefaultsToMain(t *testing.T) {
	t.Parallel()

	provider := NewGitResourceRepository(
		config.GitRepository{
			Local:  config.GitLocal{BaseDir: t.TempDir()},
			Remote: &config.GitRemote{URL: "https://example.invalid/repo.git"},
		},
		config.ResourceFormatJSON,
	)

	if got := provider.targetBranch(); got != "main" {
		t.Fatalf("expected main default branch, got %q", got)
	}
}

func TestGitRepositoryAuthMethodSanity(t *testing.T) {
	t.Parallel()

	basicProvider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: t.TempDir()},
			Remote: &config.GitRemote{
				URL: "https://example.invalid/repo.git",
				Auth: &config.GitAuth{
					BasicAuth: &config.BasicAuth{Username: "u", Password: "p"},
				},
			},
		},
		config.ResourceFormatJSON,
	)
	basicAuth, err := basicProvider.authMethod()
	if err != nil {
		t.Fatalf("authMethod basic returned error: %v", err)
	}
	if basicAuth == nil {
		t.Fatal("expected non-nil basic auth method")
	}

	tokenProvider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: t.TempDir()},
			Remote: &config.GitRemote{
				URL: "https://example.invalid/repo.git",
				Auth: &config.GitAuth{
					AccessKey: &config.AccessKeyAuth{Token: "token"},
				},
			},
		},
		config.ResourceFormatJSON,
	)
	tokenAuth, err := tokenProvider.authMethod()
	if err != nil {
		t.Fatalf("authMethod token returned error: %v", err)
	}
	if tokenAuth == nil {
		t.Fatal("expected non-nil token auth method")
	}
}

func TestGitRepositorySyncStatusStates(t *testing.T) {
	t.Parallel()

	remoteDir := createRemoteWithMainCommit(t)
	localDir := cloneMainBranch(t, remoteDir)

	provider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: localDir},
			Remote: &config.GitRemote{
				URL:    remoteDir,
				Branch: "main",
			},
		},
		config.ResourceFormatJSON,
	)

	status, err := provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus up_to_date returned error: %v", err)
	}
	if status.State != repository.SyncStateUpToDate {
		t.Fatalf("expected up_to_date, got %q", status.State)
	}

	// Uncommitted local change.
	if err := os.WriteFile(filepath.Join(localDir, "dirty.txt"), []byte("dirty"), 0o600); err != nil {
		t.Fatalf("failed to write uncommitted file: %v", err)
	}
	status, err = provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus dirty returned error: %v", err)
	}
	if !status.HasUncommitted {
		t.Fatal("expected hasUncommitted=true")
	}

	localRepo, err := gogit.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("failed to open local repo: %v", err)
	}
	commitFile(t, localRepo, localDir, "ahead.txt", "ahead", "ahead commit")

	status, err = provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus ahead returned error: %v", err)
	}
	if status.State != repository.SyncStateAhead {
		t.Fatalf("expected ahead, got %q", status.State)
	}

	peerDir := cloneMainBranch(t, remoteDir)
	peerRepo, err := gogit.PlainOpen(peerDir)
	if err != nil {
		t.Fatalf("failed to open peer repo: %v", err)
	}
	commitFile(t, peerRepo, peerDir, "peer.txt", "peer", "peer commit")
	pushCurrentBranchToMain(t, peerRepo)

	status, err = provider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus diverged returned error: %v", err)
	}
	if status.State != repository.SyncStateDiverged {
		t.Fatalf("expected diverged, got %q", status.State)
	}

	behindDir := cloneMainBranch(t, remoteDir)
	behindProvider := NewGitResourceRepository(
		config.GitRepository{
			Local: config.GitLocal{BaseDir: behindDir},
			Remote: &config.GitRemote{
				URL:    remoteDir,
				Branch: "main",
			},
		},
		config.ResourceFormatJSON,
	)

	commitFile(t, peerRepo, peerDir, "peer2.txt", "peer2", "peer second commit")
	pushCurrentBranchToMain(t, peerRepo)

	behindStatus, err := behindProvider.SyncStatus(context.Background())
	if err != nil {
		t.Fatalf("SyncStatus behind returned error: %v", err)
	}
	if behindStatus.State != repository.SyncStateBehind {
		t.Fatalf("expected behind, got %q", behindStatus.State)
	}
}

func createRemoteWithMainCommit(t *testing.T) string {
	t.Helper()

	remoteDir := t.TempDir()
	if _, err := gogit.PlainInit(remoteDir, true); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}

	seedDir := t.TempDir()
	seedRepo, err := gogit.PlainInit(seedDir, false)
	if err != nil {
		t.Fatalf("failed to init seed repo: %v", err)
	}
	commitFile(t, seedRepo, seedDir, "seed.txt", "seed", "seed commit")

	if _, err := seedRepo.CreateRemote(&gitcfg.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatalf("failed to create seed remote: %v", err)
	}

	head, err := seedRepo.Head()
	if err != nil {
		t.Fatalf("failed to resolve seed head: %v", err)
	}

	if err := seedRepo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/main", head.Name().Short())),
		},
	}); err != nil {
		t.Fatalf("failed to push seed commit: %v", err)
	}

	return remoteDir
}

func cloneMainBranch(t *testing.T, remoteDir string) string {
	t.Helper()

	localDir := t.TempDir()
	_, err := gogit.PlainClone(localDir, false, &gogit.CloneOptions{
		URL:           remoteDir,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err != nil {
		t.Fatalf("failed to clone remote: %v", err)
	}
	return localDir
}

func assertGitRepoInitialized(t *testing.T, repoDir string) {
	t.Helper()

	info, err := os.Stat(filepath.Join(repoDir, ".git"))
	if err != nil {
		t.Fatalf("expected initialized git repo at %q, stat error: %v", repoDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected .git directory at %q", repoDir)
	}
}

func assertGitRepoNotInitialized(t *testing.T, repoDir string) {
	t.Helper()

	_, err := os.Stat(filepath.Join(repoDir, ".git"))
	if err == nil {
		t.Fatalf("did not expect initialized git repo at %q", repoDir)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing .git directory at %q, got error: %v", repoDir, err)
	}
}

func commitFile(t *testing.T, repo *gogit.Repository, repoDir string, filename string, content string, message string) {
	t.Helper()

	path := filepath.Join(repoDir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create commit directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write commit file: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to open worktree: %v", err)
	}
	if _, err := worktree.Add(filename); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := worktree.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "declarest-test",
			Email: "declarest@example.com",
			When:  time.Unix(0, 0),
		},
	}); err != nil {
		t.Fatalf("failed to commit file: %v", err)
	}
}

func pushCurrentBranchToMain(t *testing.T, repo *gogit.Repository) {
	t.Helper()

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to resolve head branch: %v", err)
	}
	if err := repo.Push(&gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/main", head.Name().Short())),
		},
	}); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		t.Fatalf("failed to push peer commit: %v", err)
	}
}

func assertCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}
	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != category {
		t.Fatalf("expected %q category, got %q", category, typed.Category)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
