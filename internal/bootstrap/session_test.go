// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/crmarques/declarest/resource"
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

	if _, ok := session.Contexts.(*configfile.Service); !ok {
		t.Fatalf("expected configfile.Service, got %T", session.Contexts)
	}
	if _, ok := session.Orchestrator.(*internalorchestrator.Orchestrator); !ok {
		t.Fatalf("expected Orchestrator, got %T", session.Orchestrator)
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

	if err := session.Orchestrator.Save(context.Background(), "/customers/acme", resource.Content{Value: map[string]any{"name": "ACME"}}); err != nil {
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
        baseDir: ` + devRepo + `
    managedService:
      http:
        url: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: dev-token
  - name: prod
    repository:
      filesystem:
        baseDir: ` + prodRepo + `
    managedService:
      http:
        url: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: prod-token
currentContext: dev
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
    managedService:
      http:
        url: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: dev-token
    metadata:
      baseDir: ` + metadataDir + `
currentContext: remote-only
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
	if session.Services.ManagedServiceClient() == nil {
		t.Fatal("expected managed service to be configured")
	}
	if session.Services.MetadataService() == nil {
		t.Fatal("expected metadata service when metadata.baseDir is configured")
	}
}

func TestNewSessionAllowsRepositoryOnlyContextWithoutManagedService(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	contextCatalogPath := filepath.Join(tempDir, "contexts.yaml")
	contextCatalog := []byte(`
contexts:
  - name: local-only
    repository:
      filesystem:
        baseDir: ` + repoDir + `
currentContext: local-only
`)
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "local-only"},
	)
	if err != nil {
		t.Fatalf("expected repository-only context to bootstrap, got error: %v", err)
	}
	if session.Orchestrator == nil {
		t.Fatal("expected orchestrator")
	}
	if session.Services.RepositoryStore() == nil {
		t.Fatal("expected repository store to be configured")
	}
	if session.Services.ManagedServiceClient() != nil {
		t.Fatalf("expected nil managed service client, got %T", session.Services.ManagedServiceClient())
	}
	if session.Services.MetadataService() == nil {
		t.Fatal("expected metadata service to default from repository baseDir")
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
        baseDir: ` + filepath.Join(tempDir, "repo") + `
    managedService:
      http:
        url: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: dev-token
    metadata:
      bundle: ` + bundlePath + `
currentContext: bundled
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
	if session.Services.ManagedServiceClient() == nil {
		t.Fatal("expected managed service when managedService is configured")
	}
	openAPISpec, openAPIErr := session.Services.ManagedServiceClient().GetOpenAPISpec(context.Background())
	if openAPIErr != nil {
		t.Fatalf("expected OpenAPI to fallback from bundle, got error: %v", openAPIErr)
	}
	specMap, ok := openAPISpec.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
	}
	if specMap["openapi"] != "3.0.0" {
		t.Fatalf("expected bundled OpenAPI version 3.0.0, got %v", specMap["openapi"])
	}
}

func TestNewSessionSupportsMetadataBundleFile(t *testing.T) {
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
  - name: bundled-file
    repository:
      filesystem:
        baseDir: ` + filepath.Join(tempDir, "repo") + `
    managedService:
      http:
        url: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: dev-token
    metadata:
      bundleFile: ` + bundlePath + `
currentContext: bundled-file
`)
	if err := os.WriteFile(contextCatalogPath, contextCatalog, 0o600); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	session, err := NewSession(
		BootstrapConfig{ContextCatalogPath: contextCatalogPath},
		config.ContextSelection{Name: "bundled-file"},
	)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session.Services.MetadataService() == nil {
		t.Fatal("expected metadata service when metadata.bundleFile is configured")
	}
	if session.Services.ManagedServiceClient() == nil {
		t.Fatal("expected managed service when managedService is configured")
	}
	openAPISpec, openAPIErr := session.Services.ManagedServiceClient().GetOpenAPISpec(context.Background())
	if openAPIErr != nil {
		t.Fatalf("expected OpenAPI to fallback from bundleFile, got error: %v", openAPIErr)
	}
	specMap, ok := openAPISpec.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
	}
	if specMap["openapi"] != "3.0.0" {
		t.Fatalf("expected bundled OpenAPI version 3.0.0, got %v", specMap["openapi"])
	}
}

func TestNewSessionFromResolvedContext(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	session, err := NewSessionFromResolvedContext(config.Context{
		Name: "operator",
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{
				BaseDir: repoDir,
			},
		},
		ManagedService: &config.ManagedService{
			HTTP: &config.HTTPServer{
				BaseURL: "https://example.com/api",
				Auth: &config.HTTPAuth{
					CustomHeaders: []config.HeaderTokenAuth{{
						Header: "Authorization",
						Prefix: "Bearer",
						Value:  "operator-token",
					}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewSessionFromResolvedContext returned error: %v", err)
	}
	if session.Orchestrator == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if session.Services == nil {
		t.Fatal("expected non-nil services accessor")
	}
	if session.Contexts != nil {
		t.Fatalf("expected nil context service for resolved-context session, got %T", session.Contexts)
	}
}
