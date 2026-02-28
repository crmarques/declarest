package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	httpserver "github.com/crmarques/declarest/internal/providers/server/http"
)

func TestBuildDefaultOrchestratorWiring(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	t.Run("filesystem_context_without_optional_managers", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "fs",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{
			Name:      "fs",
			Overrides: map[string]string{"repository.filesystem.base-dir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Metadata.(*fsmetadata.FSMetadataService); !ok {
			t.Fatalf("expected FSMetadataService, got %T", defaultOrchestrator.Metadata)
		}
		if defaultOrchestrator.Server != nil {
			t.Fatalf("expected nil server manager, got %T", defaultOrchestrator.Server)
		}
		if defaultOrchestrator.Secrets != nil {
			t.Fatalf("expected nil secrets provider, got %T", defaultOrchestrator.Secrets)
		}
	})

	t.Run("metadata_bundle_local_archive", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		archivePath := filepath.Join(tempDir, "keycloak-bundle-0.0.11.tar.gz")
		writeBundleArchiveForTest(t, archivePath, map[string]string{
			"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.11
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

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "bundle",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: filepath.Join(tempDir, "repo")},
				},
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							BearerToken: &config.BearerTokenAuth{Token: "token"},
						},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}
		if _, ok := defaultOrchestrator.Metadata.(*fsmetadata.FSMetadataService); !ok {
			t.Fatalf("expected FSMetadataService for bundle metadata source, got %T", defaultOrchestrator.Metadata)
		}
		if defaultOrchestrator.Server == nil {
			t.Fatal("expected server manager")
		}
		openAPISpec, openAPIErr := defaultOrchestrator.Server.GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected OpenAPI from bundled openapi.yaml, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
		}
		if specMap["openapi"] != "3.0.0" {
			t.Fatalf("expected bundled openapi version 3.0.0, got %v", specMap["openapi"])
		}
	})

	t.Run("metadata_bundle_manifest_openapi_url", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		openAPIServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			_, _ = fmt.Fprint(w, "openapi: 3.0.2\npaths: {}\n")
		}))
		t.Cleanup(openAPIServer.Close)

		archivePath := filepath.Join(tempDir, "keycloak-bundle-0.0.12.tar.gz")
		writeBundleArchiveForTest(t, archivePath, map[string]string{
			"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.12
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak-bundle
  metadataRoot: metadata
  openapi: ` + openAPIServer.URL + `/openapi.yaml
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
			"metadata/admin/realms/_/metadata.json": `{}`,
		})

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "bundle",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: filepath.Join(tempDir, "repo")},
				},
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							BearerToken: &config.BearerTokenAuth{Token: "token"},
						},
						TLS: &config.TLS{InsecureSkipVerify: true},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}
		if defaultOrchestrator.Server == nil {
			t.Fatal("expected server manager")
		}

		openAPISpec, openAPIErr := defaultOrchestrator.Server.GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected OpenAPI from bundle manifest URL, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
		}
		if specMap["openapi"] != "3.0.2" {
			t.Fatalf("expected bundle manifest OpenAPI version 3.0.2, got %v", specMap["openapi"])
		}
	})

	t.Run("context_openapi_has_precedence_over_bundle_openapi", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		contextOpenAPIPath := filepath.Join(tempDir, "context-openapi.yaml")
		if err := os.WriteFile(contextOpenAPIPath, []byte("openapi: 3.0.1\npaths: {}\n"), 0o600); err != nil {
			t.Fatalf("failed to write context openapi file: %v", err)
		}

		archivePath := filepath.Join(tempDir, "keycloak-bundle-0.0.13.tar.gz")
		writeBundleArchiveForTest(t, archivePath, map[string]string{
			"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.13
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak-bundle
  metadataRoot: metadata
  openapi: https://www.keycloak.org/docs-api/26.4.7/rest-api/openapi.yaml
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
			"metadata/admin/realms/_/metadata.json": `{}`,
		})

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "bundle",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: filepath.Join(tempDir, "repo")},
				},
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						OpenAPI: contextOpenAPIPath,
						Auth: &config.HTTPAuth{
							BearerToken: &config.BearerTokenAuth{Token: "token"},
						},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}
		if defaultOrchestrator.Server == nil {
			t.Fatal("expected server manager")
		}

		openAPISpec, openAPIErr := defaultOrchestrator.Server.GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected context openapi source to remain valid, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
		}
		if specMap["openapi"] != "3.0.1" {
			t.Fatalf("expected context OpenAPI version 3.0.1, got %v", specMap["openapi"])
		}
	})

	t.Run("git_context_with_http_server_and_file_secret_store", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "git-http-file-secret",
				Repository: config.Repository{
					Git: &config.GitRepository{
						Local: config.GitLocal{BaseDir: "/tmp/repo"},
					},
				},
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							BearerToken: &config.BearerTokenAuth{Token: "token"},
						},
					},
				},
				SecretStore: &config.SecretStore{
					File: &config.FileSecretStore{
						Path:       "/tmp/secrets.enc",
						Passphrase: "change-me",
					},
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "git-http-file-secret"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*gitrepository.GitResourceRepository); !ok {
			t.Fatalf("expected GitResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Server.(*httpserver.HTTPResourceServerGateway); !ok {
			t.Fatalf("expected HTTPResourceServerGateway, got %T", defaultOrchestrator.Server)
		}
		if _, ok := defaultOrchestrator.Secrets.(*filesecrets.FileSecretService); !ok {
			t.Fatalf("expected FileSecretService, got %T", defaultOrchestrator.Secrets)
		}
	})

	t.Run("filesystem_context_with_vault_secret_store", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "fs-vault-secret",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				SecretStore: &config.SecretStore{
					Vault: &config.VaultSecretStore{
						Address: "https://vault.example.com",
						Auth: &config.VaultAuth{
							Token: "root-token",
						},
					},
				},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "fs-vault-secret"})
		if err != nil {
			t.Fatalf("buildDefaultOrchestrator returned error: %v", err)
		}

		if _, ok := defaultOrchestrator.Repository.(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", defaultOrchestrator.Repository)
		}
		if _, ok := defaultOrchestrator.Secrets.(*vaultsecrets.VaultSecretService); !ok {
			t.Fatalf("expected VaultSecretService, got %T", defaultOrchestrator.Secrets)
		}
	})
}

func TestEffectiveOpenAPISource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configOpenAPI   string
		metadataOpenAPI string
		want            string
	}{
		{
			name:            "context_openapi_has_precedence",
			configOpenAPI:   "/tmp/context-openapi.yaml",
			metadataOpenAPI: "/tmp/bundle-openapi.yaml",
			want:            "/tmp/context-openapi.yaml",
		},
		{
			name:            "bundle_openapi_used_when_context_is_empty",
			configOpenAPI:   "   ",
			metadataOpenAPI: "/tmp/bundle-openapi.yaml",
			want:            "/tmp/bundle-openapi.yaml",
		},
		{
			name:            "empty_when_both_sources_empty",
			configOpenAPI:   "",
			metadataOpenAPI: "   ",
			want:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := effectiveOpenAPISource(tt.configOpenAPI, tt.metadataOpenAPI)
			if got != tt.want {
				t.Fatalf("expected openapi source %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildDefaultOrchestratorValidationAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil_context_service", func(t *testing.T) {
		t.Parallel()

		_, err := buildDefaultOrchestrator(context.Background(), nil, config.ContextSelection{})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("context_service_error_is_propagated", func(t *testing.T) {
		t.Parallel()

		expected := faults.NewTypedError(faults.NotFoundError, "context not found", nil)
		contextService := &fakeContextService{resolveErr: expected}

		_, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "missing"})
		if !errors.Is(err, expected) {
			t.Fatalf("expected propagated error %v, got %v", expected, err)
		}
	})

	t.Run("missing_repository_provider_is_allowed", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name:       "invalid",
				Repository: config.Repository{},
			},
		}

		defaultOrchestrator, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid"})
		if err != nil {
			t.Fatalf("expected missing repository to be allowed, got error: %v", err)
		}
		if defaultOrchestrator.Repository != nil {
			t.Fatalf("expected nil repository manager, got %T", defaultOrchestrator.Repository)
		}
	})

	t.Run("invalid_resource_server_provider_configuration", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "invalid-resource-server",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com/api",
						Auth: &config.HTTPAuth{
							OAuth2: &config.OAuth2{
								TokenURL:     "https://example.com/oauth/token",
								GrantType:    "password",
								ClientID:     "id",
								ClientSecret: "secret",
							},
						},
					},
				},
			},
		}

		_, err := buildDefaultOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid-resource-server"})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})
}

type fakeContextService struct {
	resolvedContext config.Context
	resolveErr      error
}

func (s *fakeContextService) Create(context.Context, config.Context) error {
	return nil
}

func (s *fakeContextService) Update(context.Context, config.Context) error {
	return nil
}

func (s *fakeContextService) Delete(context.Context, string) error {
	return nil
}

func (s *fakeContextService) Rename(context.Context, string, string) error {
	return nil
}

func (s *fakeContextService) List(context.Context) ([]config.Context, error) {
	return nil, nil
}

func (s *fakeContextService) SetCurrent(context.Context, string) error {
	return nil
}

func (s *fakeContextService) GetCurrent(context.Context) (config.Context, error) {
	return config.Context{}, nil
}

func (s *fakeContextService) ResolveContext(context.Context, config.ContextSelection) (config.Context, error) {
	if s.resolveErr != nil {
		return config.Context{}, s.resolveErr
	}
	return s.resolvedContext, nil
}

func (s *fakeContextService) Validate(context.Context, config.Context) error {
	return nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}

func writeBundleArchiveForTest(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	buffer := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for path, content := range files {
		data := []byte(content)
		header := &tar.Header{
			Name: filepath.ToSlash(path),
			Mode: 0o644,
			Size: int64(len(data)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header for %q: %v", path, err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatalf("failed to write tar data for %q: %v", path, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	if err := os.WriteFile(archivePath, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive %q: %v", archivePath, err)
	}
}
