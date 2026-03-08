package fsstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestLocalResourceRepositorySaveRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "customers")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	repo := NewLocalResourceRepository(root)
	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{"name": "ACME"},
	})
	if err == nil {
		t.Fatal("expected save to reject symlink escape path")
	}
	if !strings.Contains(err.Error(), "escapes repository base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
