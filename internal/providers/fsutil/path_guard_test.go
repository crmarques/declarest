package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPathUnderRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Clean("/tmp/root")
	if !IsPathUnderRoot(root, filepath.Join(root, "a", "b")) {
		t.Fatal("expected child path to be under root")
	}
	if IsPathUnderRoot(root, filepath.Clean("/tmp/other/file")) {
		t.Fatal("expected unrelated path to be outside root")
	}
}

func TestIsPathUnderRootRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	candidate := filepath.Join(linkPath, "escaped.txt")
	if IsPathUnderRoot(root, candidate) {
		t.Fatalf("expected symlinked path %q to be rejected under root %q", candidate, root)
	}
}

func TestCleanupEmptyParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	leaf := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatalf("failed to create leaf dir: %v", err)
	}

	if err := CleanupEmptyParents(leaf, root); err != nil {
		t.Fatalf("CleanupEmptyParents returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("expected parent to be removed, got err=%v", err)
	}
}
