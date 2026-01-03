package repository

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func commitFile(t *testing.T, repo *git.Repository, dir, name, content string) plumbing.Hash {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if _, err := wt.Add(name); err != nil {
		t.Fatalf("add file: %v", err)
	}

	sig := &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Now(),
	}

	hash, err := wt.Commit("commit "+name, &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return hash
}

func setupRemoteRepo(t *testing.T) (string, *git.Repository, string, plumbing.Hash) {
	t.Helper()

	remoteDir := t.TempDir()
	if _, err := git.PlainInit(remoteDir, true); err != nil {
		t.Fatalf("init remote: %v", err)
	}

	seedDir := t.TempDir()
	seedRepo, err := git.PlainInit(seedDir, false)
	if err != nil {
		t.Fatalf("init seed: %v", err)
	}

	seedHash := commitFile(t, seedRepo, seedDir, "seed.txt", "one")

	if _, err := seedRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push seed: %v", err)
	}

	return remoteDir, seedRepo, seedDir, seedHash
}

func TestGitRepositoryManagerRebaseLocalFromRemote(t *testing.T) {
	remoteDir, seedRepo, seedDir, _ := setupRemoteRepo(t)

	localDir := t.TempDir()
	if _, err := git.PlainClone(localDir, false, &git.CloneOptions{URL: remoteDir}); err != nil {
		t.Fatalf("clone: %v", err)
	}

	newHash := commitFile(t, seedRepo, seedDir, "seed.txt", "two")
	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		t.Fatalf("push update: %v", err)
	}

	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	if err := manager.RebaseLocalFromRemote(); err != nil {
		t.Fatalf("RebaseLocalFromRemote: %v", err)
	}

	repo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Hash() != newHash {
		t.Fatalf("expected local head %s, got %s", newHash, head.Hash())
	}
}

func TestGitRepositoryManagerRebaseLocalFromRemoteClonesMissingRepo(t *testing.T) {
	remoteDir, _, _, seedHash := setupRemoteRepo(t)

	localDir := t.TempDir()

	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	if err := manager.RebaseLocalFromRemote(); err != nil {
		t.Fatalf("RebaseLocalFromRemote: %v", err)
	}

	repo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Hash() != seedHash {
		t.Fatalf("expected local head %s, got %s", seedHash, head.Hash())
	}
}

func TestGitRepositoryManagerPushLocalDiffsToRemote(t *testing.T) {
	remoteDir, _, _, _ := setupRemoteRepo(t)

	localDir := t.TempDir()
	if _, err := git.PlainClone(localDir, false, &git.CloneOptions{URL: remoteDir}); err != nil {
		t.Fatalf("clone: %v", err)
	}

	localRepo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	newHash := commitFile(t, localRepo, localDir, "local.txt", "local")

	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	if err := manager.PushLocalDiffsToRemote(); err != nil {
		t.Fatalf("PushLocalDiffsToRemote: %v", err)
	}

	remoteRepo, err := git.PlainOpen(remoteDir)
	if err != nil {
		t.Fatalf("open remote repo: %v", err)
	}
	head, err := localRepo.Head()
	if err != nil {
		t.Fatalf("local head: %v", err)
	}
	branch := head.Name().Short()
	remoteRef, err := remoteRepo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		t.Fatalf("remote ref: %v", err)
	}
	if remoteRef.Hash() != newHash {
		t.Fatalf("expected remote head %s, got %s", newHash, remoteRef.Hash())
	}
}

func TestGitRepositoryManagerForcePushLocalDiffsToRemote(t *testing.T) {
	remoteDir, seedRepo, seedDir, _ := setupRemoteRepo(t)

	localDir := t.TempDir()
	if _, err := git.PlainClone(localDir, false, &git.CloneOptions{URL: remoteDir}); err != nil {
		t.Fatalf("clone: %v", err)
	}

	_ = commitFile(t, seedRepo, seedDir, "remote.txt", "remote")
	if err := seedRepo.Push(&git.PushOptions{RemoteName: "origin"}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		t.Fatalf("push remote: %v", err)
	}

	localRepo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	localHash := commitFile(t, localRepo, localDir, "local.txt", "local")

	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	if err := manager.PushLocalDiffsToRemote(); err == nil {
		t.Fatalf("expected non-fast-forward push failure")
	}

	if err := manager.ForcePushLocalDiffsToRemote(); err != nil {
		t.Fatalf("ForcePushLocalDiffsToRemote: %v", err)
	}

	remoteRepo, err := git.PlainOpen(remoteDir)
	if err != nil {
		t.Fatalf("open remote repo: %v", err)
	}
	head, err := localRepo.Head()
	if err != nil {
		t.Fatalf("local head: %v", err)
	}
	branch := head.Name().Short()
	remoteRef, err := remoteRepo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		t.Fatalf("remote ref: %v", err)
	}
	if remoteRef.Hash() != localHash {
		t.Fatalf("expected remote head %s, got %s", localHash, remoteRef.Hash())
	}
}

func TestGitRepositoryManagerResetLocal(t *testing.T) {
	remoteDir, _, _, seedHash := setupRemoteRepo(t)

	localDir := t.TempDir()
	if _, err := git.PlainClone(localDir, false, &git.CloneOptions{URL: remoteDir}); err != nil {
		t.Fatalf("clone: %v", err)
	}

	localRepo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	_ = commitFile(t, localRepo, localDir, "local.txt", "local")

	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: remoteDir,
		},
	})

	if err := manager.ResetLocal(); err != nil {
		t.Fatalf("ResetLocal: %v", err)
	}

	repo, err := git.PlainOpen(localDir)
	if err != nil {
		t.Fatalf("open local repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Hash() != seedHash {
		t.Fatalf("expected local head %s, got %s", seedHash, head.Hash())
	}
}

func TestAuthFromConfigAccessKeyProviderGitHub(t *testing.T) {
	cfg := &GitResourceRepositoryRemoteConfig{
		Provider: "GitHub",
		Auth: &GitResourceRepositoryRemoteAuthConfig{
			AccessKey: &GitResourceRepositoryAccessKeyConfig{
				Token: "token-123",
			},
		},
	}

	auth, err := authFromConfig(cfg, "https://github.com/org/repo.git")
	if err != nil {
		t.Fatalf("authFromConfig: %v", err)
	}
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected BasicAuth, got %T", auth)
	}
	if basic.Username != "x-access-token" {
		t.Fatalf("expected username x-access-token, got %q", basic.Username)
	}
	if basic.Password != "token-123" {
		t.Fatalf("expected password token-123, got %q", basic.Password)
	}
}

func TestAuthFromConfigAccessKeyProviderGitLab(t *testing.T) {
	cfg := &GitResourceRepositoryRemoteConfig{
		Provider: "gitlab",
		Auth: &GitResourceRepositoryRemoteAuthConfig{
			AccessKey: &GitResourceRepositoryAccessKeyConfig{
				Token: "token-456",
			},
		},
	}

	auth, err := authFromConfig(cfg, "https://gitlab.com/org/repo.git")
	if err != nil {
		t.Fatalf("authFromConfig: %v", err)
	}
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected BasicAuth, got %T", auth)
	}
	if basic.Username != "oauth2" {
		t.Fatalf("expected username oauth2, got %q", basic.Username)
	}
	if basic.Password != "token-456" {
		t.Fatalf("expected password token-456, got %q", basic.Password)
	}
}

func TestAuthFromConfigAccessKeyProviderGitea(t *testing.T) {
	cfg := &GitResourceRepositoryRemoteConfig{
		Provider: "gitea",
		Auth: &GitResourceRepositoryRemoteAuthConfig{
			AccessKey: &GitResourceRepositoryAccessKeyConfig{
				Token: "token-789",
			},
		},
	}

	auth, err := authFromConfig(cfg, "https://gitea.example.com/org/repo.git")
	if err != nil {
		t.Fatalf("authFromConfig: %v", err)
	}
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected BasicAuth, got %T", auth)
	}
	if basic.Username != "token" {
		t.Fatalf("expected username token, got %q", basic.Username)
	}
	if basic.Password != "token-789" {
		t.Fatalf("expected password token-789, got %q", basic.Password)
	}
}

func TestAuthFromConfigAccessKeyDefaultBearer(t *testing.T) {
	cfg := &GitResourceRepositoryRemoteConfig{
		Auth: &GitResourceRepositoryRemoteAuthConfig{
			AccessKey: &GitResourceRepositoryAccessKeyConfig{
				Token: "token-789",
			},
		},
	}

	auth, err := authFromConfig(cfg, "")
	if err != nil {
		t.Fatalf("authFromConfig: %v", err)
	}
	if _, ok := auth.(*githttp.TokenAuth); !ok {
		t.Fatalf("expected TokenAuth, got %T", auth)
	}
}

func TestAuthFromConfigRejectsUnknownProvider(t *testing.T) {
	cfg := &GitResourceRepositoryRemoteConfig{
		Provider: "bitbucket",
	}

	if _, err := authFromConfig(cfg, ""); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestGitRepositoryManagerSyncLocalFromRemoteProvidesHints(t *testing.T) {
	localDir := t.TempDir()
	manager := NewGitResourceRepositoryManager(localDir)
	manager.SetConfig(&GitResourceRepositoryConfig{
		Remote: &GitResourceRepositoryRemoteConfig{
			URL: filepath.Join(localDir, "missing-remote"),
		},
	})

	err := manager.SyncLocalFromRemoteIfConfigured()
	if err == nil {
		t.Fatalf("expected sync error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "declarest repo reset") || !strings.Contains(msg, "declarest repo push --force") {
		t.Fatalf("expected recovery hints in error, got %q", msg)
	}
}
