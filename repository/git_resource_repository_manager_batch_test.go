package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/crmarques/declarest/resource"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGitRepositoryManagerRunBatchCommitsOnce(t *testing.T) {
	dir := t.TempDir()
	manager := NewGitResourceRepositoryManager(dir)

	first, err := resource.NewResource(map[string]any{"id": "one"})
	if err != nil {
		t.Fatalf("new resource one: %v", err)
	}
	second, err := resource.NewResource(map[string]any{"id": "two"})
	if err != nil {
		t.Fatalf("new resource two: %v", err)
	}

	if err := manager.RunBatch(func() error {
		if err := manager.ApplyResource("/items/one", first); err != nil {
			return err
		}
		if err := manager.ApplyResource("/items/two", second); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	if got := countCommits(t, repo); got != 1 {
		t.Fatalf("commit count = %d, want 1", got)
	}
}

func TestGitRepositoryManagerApplyResourceCommitsPerCall(t *testing.T) {
	dir := t.TempDir()
	manager := NewGitResourceRepositoryManager(dir)

	for idx := 1; idx <= 2; idx++ {
		res, err := resource.NewResource(map[string]any{"id": fmt.Sprintf("%d", idx)})
		if err != nil {
			t.Fatalf("new resource %d: %v", idx, err)
		}
		path := fmt.Sprintf("/items/%d", idx)
		if err := manager.ApplyResource(path, res); err != nil {
			t.Fatalf("ApplyResource %s: %v", path, err)
		}
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	if got := countCommits(t, repo); got != 2 {
		t.Fatalf("commit count = %d, want 2", got)
	}
}

func countCommits(t *testing.T, repo *git.Repository) int {
	t.Helper()
	head, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return 0
		}
		t.Fatalf("repo head: %v", err)
	}
	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		t.Fatalf("repo log: %v", err)
	}
	defer iter.Close()

	count := 0
	if err := iter.ForEach(func(_ *object.Commit) error {
		count++
		return nil
	}); err != nil {
		t.Fatalf("iterate commits: %v", err)
	}
	return count
}
