package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestEnsureRepositoryOverwriteAllowedUsesResolvedAliasPath(t *testing.T) {
	repoDir := t.TempDir()
	repo := repository.NewGitResourceRepositoryManager(repoDir)
	if err := repo.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"aliasFromAttribute":"name"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}

	existing, err := resource.NewResource(map[string]any{"name": "existing"})
	if err != nil {
		t.Fatalf("NewResource existing: %v", err)
	}
	if err := repo.ApplyResource("/items/existing", existing); err != nil {
		t.Fatalf("ApplyResource existing: %v", err)
	}

	recon := &reconciler.DefaultReconciler{
		ResourceRepositoryManager: repo,
	}
	recon.ResourceRecordProvider = repository.NewDefaultResourceRecordProvider(repoDir, recon)

	incoming, err := resource.NewResource(map[string]any{"name": "existing"})
	if err != nil {
		t.Fatalf("NewResource incoming: %v", err)
	}

	err = ensureRepositoryOverwriteAllowed(recon, "/items/input-name", incoming, false)
	if err == nil {
		t.Fatal("expected overwrite guard error")
	}
	if !strings.Contains(err.Error(), "/items/existing") {
		t.Fatalf("expected resolved alias path in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "resolved from /items/input-name") {
		t.Fatalf("expected original path in error, got %v", err)
	}
}
