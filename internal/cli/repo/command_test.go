package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListRepositoryTreePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRepoTreeTestFile(t, filepath.Join(root, ".git", "objects", "pack", "x"), "ignored")
	writeRepoTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "_", "metadata.yaml"), "ignored")
	writeRepoTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "resource.yaml"), "ignored")
	writeRepoTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "clients", "test", "resource.yaml"), "ignored")
	writeRepoTreeTestFile(t, filepath.Join(root, "admin", "realms", "acme", "user-registry", "AD PRD", "resource.yaml"), "ignored")
	writeRepoTreeTestFile(t, filepath.Join(root, "README.md"), "ignored")

	got, err := listRepositoryTreePaths(root)
	if err != nil {
		t.Fatalf("listRepositoryTreePaths returned error: %v", err)
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
		t.Fatalf("listRepositoryTreePaths() = %#v, want %#v", got, want)
	}
}

func writeRepoTreeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}
