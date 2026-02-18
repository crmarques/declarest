package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
)

func TestNewDeclarestContext(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "dev-repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	writeContextCatalog(t, contextCatalogPath, repoDir, repoDir)

	declarestContext, err := NewDeclarestContext(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "dev"},
	)
	if err != nil {
		t.Fatalf("NewDeclarestContext returned error: %v", err)
	}

	if declarestContext.Contexts == nil {
		t.Fatal("expected non-nil contexts service")
	}
	if declarestContext.Orchestrator == nil {
		t.Fatal("expected non-nil resource reconciler")
	}

	if _, ok := declarestContext.Contexts.(*configfile.FileContextService); !ok {
		t.Fatalf("expected FileContextService, got %T", declarestContext.Contexts)
	}
	if _, ok := declarestContext.Orchestrator.(*orchestratordomain.DefaultOrchestrator); !ok {
		t.Fatalf("expected DefaultOrchestrator, got %T", declarestContext.Orchestrator)
	}
}

func TestNewDeclarestContextUsesContextCatalogPathAndSelection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	devRepo := filepath.Join(tempDir, "dev-repo")
	prodRepo := filepath.Join(tempDir, "prod-repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")

	writeContextCatalog(t, contextCatalogPath, devRepo, prodRepo)

	declarestContext, err := NewDeclarestContext(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "prod"},
	)
	if err != nil {
		t.Fatalf("NewDeclarestContext returned error: %v", err)
	}

	if err := declarestContext.Orchestrator.Save(context.Background(), "/customers/acme", map[string]any{"name": "ACME"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	prodPath := filepath.Join(prodRepo, "customers", "acme", "resource.json")
	if _, err := os.Stat(prodPath); err != nil {
		t.Fatalf("expected resource in selected context repository %q: %v", prodPath, err)
	}

	devPath := filepath.Join(devRepo, "customers", "acme", "resource.json")
	if _, err := os.Stat(devPath); err == nil {
		t.Fatalf("resource should not be written to non-selected repository %q", devPath)
	}
}

func writeContextCatalog(t *testing.T, path string, devRepo string, prodRepo string) {
	t.Helper()

	contextCatalog := []byte(`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: ` + devRepo + `
  - name: prod
    repository:
      filesystem:
        base-dir: ` + prodRepo + `
current-ctx: dev
`)
	if err := os.WriteFile(path, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}
}

func TestNewDeclarestContextFailsFastWhenCurrentContextMissing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	contextCatalog := []byte("contexts: []\n")
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	_, err := NewDeclarestContext(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.NotFoundError)
}
