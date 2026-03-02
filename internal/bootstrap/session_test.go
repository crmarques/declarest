package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	internalorchestrator "github.com/crmarques/declarest/internal/orchestrator"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
)

func TestNewSession(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "dev-repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	writeContextCatalog(t, contextCatalogPath, repoDir, repoDir)

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "dev"},
	)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if session.Contexts == nil {
		t.Fatal("expected non-nil contexts service")
	}
	if session.Orchestrator == nil {
		t.Fatal("expected non-nil resource orchestrator")
	}

	if _, ok := session.Contexts.(*configfile.FileContextService); !ok {
		t.Fatalf("expected FileContextService, got %T", session.Contexts)
	}
	if _, ok := session.Orchestrator.(*internalorchestrator.DefaultOrchestrator); !ok {
		t.Fatalf("expected DefaultOrchestrator, got %T", session.Orchestrator)
	}
}

func TestNewSessionUsesContextCatalogPathAndSelection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	devRepo := filepath.Join(tempDir, "dev-repo")
	prodRepo := filepath.Join(tempDir, "prod-repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")

	writeContextCatalog(t, contextCatalogPath, devRepo, prodRepo)

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "prod"},
	)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if err := session.Orchestrator.Save(context.Background(), "/customers/acme", map[string]any{"name": "ACME"}); err != nil {
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
    managed-server:
      http:
        base-url: https://example.com/api
        auth:
          custom-headers:
            - header: Authorization
              prefix: Bearer
              value: dev-token
  - name: prod
    repository:
      filesystem:
        base-dir: ` + prodRepo + `
    managed-server:
      http:
        base-url: https://example.com/api
        auth:
          custom-headers:
            - header: Authorization
              prefix: Bearer
              value: prod-token
current-ctx: dev
`)
	if err := os.WriteFile(path, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}
}

func TestNewSessionFailsFastWhenCurrentContextMissing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	contextCatalog := []byte("contexts: []\n")
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	_, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestNewSessionAllowsRemoteOnlyContextWithoutRepository(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	metadataDir := filepath.Join(tempDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	contextCatalog := []byte(`
contexts:
  - name: remote-only
    managed-server:
      http:
        base-url: https://example.com/api
        auth:
          custom-headers:
            - header: Authorization
              prefix: Bearer
              value: dev-token
    metadata:
      base-dir: ` + metadataDir + `
current-ctx: remote-only
`)
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "remote-only"},
	)
	if err != nil {
		t.Fatalf("expected remote-only context to bootstrap, got error: %v", err)
	}
	if session.Orchestrator == nil {
		t.Fatal("expected orchestrator")
	}
	if session.Services.RepositoryStore() != nil {
		t.Fatalf("expected nil repository store, got %T", session.Services.RepositoryStore())
	}
	if session.Services.ManagedServerClient() == nil {
		t.Fatal("expected managed server to be configured")
	}
	if session.Services.MetadataService() == nil {
		t.Fatal("expected metadata service when metadata.base-dir is configured")
	}
}

func TestNewSessionSupportsMetadataBundle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tempDir := t.TempDir()
	bundlePath := filepath.Join(tempDir, "keycloak-bundle-0.0.14.tar.gz")
	writeBundleArchiveForTest(t, bundlePath, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.14
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak-bundle
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
		"openapi.yaml": `
openapi: 3.0.0
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	contextCatalog := []byte(`
contexts:
  - name: bundled
    repository:
      filesystem:
        base-dir: ` + filepath.Join(tempDir, "repo") + `
    managed-server:
      http:
        base-url: https://example.com/api
        auth:
          custom-headers:
            - header: Authorization
              prefix: Bearer
              value: dev-token
    metadata:
      bundle: ` + bundlePath + `
current-ctx: bundled
`)
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "bundled"},
	)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session.Services.MetadataService() == nil {
		t.Fatal("expected metadata service when metadata.bundle is configured")
	}
	if session.Services.ManagedServerClient() == nil {
		t.Fatal("expected managed server when managed-server is configured")
	}
	openAPISpec, openAPIErr := session.Services.ManagedServerClient().GetOpenAPISpec(context.Background())
	if openAPIErr != nil {
		t.Fatalf("expected OpenAPI to fallback from bundle, got error: %v", openAPIErr)
	}
	specMap, ok := openAPISpec.(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
	}
	if specMap["openapi"] != "3.0.0" {
		t.Fatalf("expected bundled OpenAPI version 3.0.0, got %v", specMap["openapi"])
	}
}
