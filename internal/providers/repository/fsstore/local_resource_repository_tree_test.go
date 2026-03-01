package fsstore

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLocalResourceRepositoryTree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTreeTestFile(t, filepath.Join(root, ".git", "objects", "pack", "x"), "ignored")
	writeTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "_", "metadata.yaml"), "ignored")
	writeTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "resource.yaml"), "ignored")
	writeTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "clients", "test", "resource.yaml"), "ignored")
	writeTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "user-registry", "AD PRD", "resource.yaml"), "ignored")
	writeTreeTestFile(t, filepath.Join(root, "README.md"), "ignored")

	repo := NewLocalResourceRepository(root, "yaml")
	got, err := repo.Tree(context.Background())
	if err != nil {
		t.Fatalf("Tree returned error: %v", err)
	}

	want := []string{
		"admin",
		"admin/realms",
		"admin/realms/acme",
		"admin/realms/acme/clients",
		"admin/realms/acme/clients/test",
		"admin/realms/acme/user-registry",
		"admin/realms/acme/user-registry/AD PRD",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tree() = %#v, want %#v", got, want)
	}
}

func TestLocalResourceRepositoryTreeMissingBaseDir(t *testing.T) {
	t.Parallel()

	repo := NewLocalResourceRepository(filepath.Join(t.TempDir(), "missing"), "json")
	_, err := repo.Tree(context.Background())
	if err == nil {
		t.Fatal("expected error for missing base directory")
	}
}

func writeTreeTestFile(t *testing.T, filePath string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create directory for %q: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", filePath, err)
	}
}
