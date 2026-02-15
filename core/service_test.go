package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
	defaultreconciler "github.com/crmarques/declarest/internal/providers/reconciler/default"
)

func TestNewAppState(t *testing.T) {
	t.Parallel()

	appState := NewAppState(BootstrapConfig{}, config.ContextSelection{})
	if appState.Contexts == nil {
		t.Fatal("expected non-nil contexts service")
	}
	if appState.Reconciler == nil {
		t.Fatal("expected non-nil resource reconciler")
	}

	if _, ok := appState.Contexts.(*configfile.FileContextService); !ok {
		t.Fatalf("expected FileContextService, got %T", appState.Contexts)
	}
	if _, ok := appState.Reconciler.(*defaultreconciler.DefaultResourceReconciler); !ok {
		t.Fatalf("expected DefaultResourceReconciler, got %T", appState.Reconciler)
	}
}

func TestNewAppStateUsesContextCatalogPathAndSelection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	devRepo := filepath.Join(tempDir, "dev-repo")
	prodRepo := filepath.Join(tempDir, "prod-repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")

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
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	appState := NewAppState(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "prod"},
	)

	if err := appState.Reconciler.Save(context.Background(), "/customers/acme", map[string]any{"name": "ACME"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	prodPath := filepath.Join(prodRepo, "customers", "acme.json")
	if _, err := os.Stat(prodPath); err != nil {
		t.Fatalf("expected resource in selected context repository %q: %v", prodPath, err)
	}

	devPath := filepath.Join(devRepo, "customers", "acme.json")
	if _, err := os.Stat(devPath); err == nil {
		t.Fatalf("resource should not be written to non-selected repository %q", devPath)
	}
}
