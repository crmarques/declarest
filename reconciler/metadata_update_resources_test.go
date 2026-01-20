package reconciler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestUpdateLocalResourceForMetadataMovesResourceTree(t *testing.T) {
	baseDir := t.TempDir()
	repo := repository.NewFileSystemResourceRepositoryManager(baseDir)
	if err := repo.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	recon := &DefaultReconciler{
		ResourceRepositoryManager: repo,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(baseDir, recon)

	res, err := resource.NewResource(map[string]any{
		"id":   "legacy",
		"name": "pretty",
	})
	if err != nil {
		t.Fatalf("new resource: %v", err)
	}
	if err := repo.ApplyResource("/items/legacy", res); err != nil {
		t.Fatalf("apply resource: %v", err)
	}

	child, err := resource.NewResource(map[string]any{
		"id": "nested",
	})
	if err != nil {
		t.Fatalf("new child resource: %v", err)
	}
	if err := repo.ApplyResource("/items/legacy/children/foo", child); err != nil {
		t.Fatalf("apply child: %v", err)
	}

	metaPath := filepath.Join(baseDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"aliasFromAttribute":"name"}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	updatedPath, moved, err := recon.updateLocalResourceForMetadata("/items/legacy", res)
	if err != nil {
		t.Fatalf("update resource: %v", err)
	}
	if !moved {
		t.Fatalf("expected resource to be moved")
	}
	if updatedPath != "/items/pretty" {
		t.Fatalf("expected updated path /items/pretty, got %q", updatedPath)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "items", "pretty", "resource.json")); err != nil {
		t.Fatalf("expected new resource path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "items", "legacy", "resource.json")); err == nil {
		t.Fatalf("expected old resource to be moved")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "items", "pretty", "children", "foo", "resource.json")); err != nil {
		t.Fatalf("expected nested resource to be moved: %v", err)
	}
}
