package bootstrap

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
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	httpmanagedserver "github.com/crmarques/declarest/internal/providers/managedserver/http"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	fsstore "github.com/crmarques/declarest/internal/providers/repository/fsstore"
	gitrepository "github.com/crmarques/declarest/internal/providers/repository/git"
	filesecrets "github.com/crmarques/declarest/internal/providers/secrets/file"
	vaultsecrets "github.com/crmarques/declarest/internal/providers/secrets/vault"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestBuildOrchestratorWiring(t *testing.T) {
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

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{
			Name:      "fs",
			Overrides: map[string]string{"repository.filesystem.base-dir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}

		if _, ok := orch.RepositoryStore().(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", orch.RepositoryStore())
		}
		if _, ok := orch.MetadataService().(*fsmetadata.FSMetadataService); !ok {
			t.Fatalf("expected FSMetadataService, got %T", orch.MetadataService())
		}
		if orch.ManagedServerClient() != nil {
			t.Fatalf("expected nil server manager, got %T", orch.ManagedServerClient())
		}
		if orch.SecretProvider() != nil {
			t.Fatalf("expected nil secrets provider, got %T", orch.SecretProvider())
		}
	})

	t.Run("metadata_bundle_local_archive", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		archivePath := filepath.Join(tempDir, "keycloak-bundle-0.0.14.tar.gz")
		writeBundleArchiveForTest(t, archivePath, map[string]string{
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

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "bundle",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: filepath.Join(tempDir, "repo")},
				},
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}
		if _, ok := orch.MetadataService().(*fsmetadata.LayeredMetadataService); !ok {
			t.Fatalf("expected LayeredMetadataService for bundle metadata source, got %T", orch.MetadataService())
		}
		if orch.ManagedServerClient() == nil {
			t.Fatal("expected server manager")
		}
		openAPISpec, openAPIErr := orch.ManagedServerClient().GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected OpenAPI from bundled openapi.yaml, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
		}
		if specMap["openapi"] != "3.0.0" {
			t.Fatalf("expected bundled openapi version 3.0.0, got %v", specMap["openapi"])
		}
	})

	t.Run("explicit_metadata_base_dir_writes_to_shared_source", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repoDir := filepath.Join(tempDir, "repo")
		metadataDir := filepath.Join(tempDir, "metadata")

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "shared-metadata",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: repoDir},
				},
				Metadata: config.Metadata{
					BaseDir: metadataDir,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "shared-metadata"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}
		if _, ok := orch.MetadataService().(*fsmetadata.LayeredMetadataService); !ok {
			t.Fatalf("expected LayeredMetadataService for explicit metadata.baseDir, got %T", orch.MetadataService())
		}

		if err := orch.MetadataService().Set(context.Background(), "/customers/_", metadata.ResourceMetadata{
			Defaults: &metadata.DefaultsSpec{Value: metadata.DefaultsIncludePlaceholder("defaults.yaml")},
		}); err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
		artifactStore, ok := orch.MetadataService().(metadata.DefaultsArtifactStore)
		if !ok {
			t.Fatalf("expected DefaultsArtifactStore, got %T", orch.MetadataService())
		}
		if err := artifactStore.WriteDefaultsArtifact(context.Background(), "/customers/_", "defaults.yaml", resource.Content{
			Value: map[string]any{"team": "platform"},
		}); err != nil {
			t.Fatalf("WriteDefaultsArtifact returned error: %v", err)
		}

		sharedMetadataPath := filepath.Join(metadataDir, "customers", "_", "metadata.yaml")
		if _, err := os.Stat(sharedMetadataPath); err != nil {
			t.Fatalf("expected shared metadata file %q, got %v", sharedMetadataPath, err)
		}
		sharedDefaultsPath := filepath.Join(metadataDir, "customers", "_", "defaults.yaml")
		if _, err := os.Stat(sharedDefaultsPath); err != nil {
			t.Fatalf("expected shared defaults artifact %q, got %v", sharedDefaultsPath, err)
		}
		repoMetadataPath := filepath.Join(repoDir, "customers", "_", "metadata.yaml")
		if _, err := os.Stat(repoMetadataPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected repo-local metadata file %q to remain absent, got %v", repoMetadataPath, err)
		}
		repoDefaultsPath := filepath.Join(repoDir, "customers", "_", "defaults.yaml")
		if _, err := os.Stat(repoDefaultsPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected repo-local defaults artifact %q to remain absent, got %v", repoDefaultsPath, err)
		}
	})

	t.Run("metadata_bundle_file_local_archive", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		archivePath := filepath.Join(tempDir, "keycloak-bundle-0.0.15.tar.gz")
		writeBundleArchiveForTest(t, archivePath, map[string]string{
			"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.15
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
				Name: "bundle-file",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: filepath.Join(tempDir, "repo")},
				},
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
					},
				},
				Metadata: config.Metadata{
					BundleFile: archivePath,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle-file"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}
		if _, ok := orch.MetadataService().(*fsmetadata.LayeredMetadataService); !ok {
			t.Fatalf("expected LayeredMetadataService for bundle-file metadata source, got %T", orch.MetadataService())
		}
		if orch.ManagedServerClient() == nil {
			t.Fatal("expected server manager")
		}
		openAPISpec, openAPIErr := orch.ManagedServerClient().GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected OpenAPI from bundled openapi.yaml, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI map payload, got %T", openAPISpec)
		}
		if specMap["openapi"] != "3.0.0" {
			t.Fatalf("expected bundled openapi version 3.0.0, got %v", specMap["openapi"])
		}
	})

	t.Run("metadata_bundle_writes_to_repo_overlay", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repoDir := filepath.Join(tempDir, "repo")
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
			"metadata/admin/realms/_/metadata.json": `{}`,
		})

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "bundle-write-target",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: repoDir},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle-write-target"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}

		if err := orch.MetadataService().Set(context.Background(), "/customers/_", metadata.ResourceMetadata{
			Defaults: &metadata.DefaultsSpec{Value: metadata.DefaultsIncludePlaceholder("defaults.yaml")},
		}); err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
		artifactStore, ok := orch.MetadataService().(metadata.DefaultsArtifactStore)
		if !ok {
			t.Fatalf("expected DefaultsArtifactStore, got %T", orch.MetadataService())
		}
		if err := artifactStore.WriteDefaultsArtifact(context.Background(), "/customers/_", "defaults.yaml", resource.Content{
			Value: map[string]any{"team": "platform"},
		}); err != nil {
			t.Fatalf("WriteDefaultsArtifact returned error: %v", err)
		}

		repoMetadataPath := filepath.Join(repoDir, "customers", "_", "metadata.yaml")
		if _, err := os.Stat(repoMetadataPath); err != nil {
			t.Fatalf("expected repo-local metadata file %q, got %v", repoMetadataPath, err)
		}
		repoDefaultsPath := filepath.Join(repoDir, "customers", "_", "defaults.yaml")
		if _, err := os.Stat(repoDefaultsPath); err != nil {
			t.Fatalf("expected repo-local defaults artifact %q, got %v", repoDefaultsPath, err)
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
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
						TLS: &config.TLS{InsecureSkipVerify: true},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}
		if orch.ManagedServerClient() == nil {
			t.Fatal("expected server manager")
		}

		openAPISpec, openAPIErr := orch.ManagedServerClient().GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected OpenAPI from bundle manifest URL, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.Value.(map[string]any)
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
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						OpenAPI: contextOpenAPIPath,
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
					},
				},
				Metadata: config.Metadata{
					Bundle: archivePath,
				},
			},
		}

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "bundle"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}
		if orch.ManagedServerClient() == nil {
			t.Fatal("expected server manager")
		}

		openAPISpec, openAPIErr := orch.ManagedServerClient().GetOpenAPISpec(context.Background())
		if openAPIErr != nil {
			t.Fatalf("expected context openapi source to remain valid, got error: %v", openAPIErr)
		}
		specMap, ok := openAPISpec.Value.(map[string]any)
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
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
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

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "git-http-file-secret"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}

		if _, ok := orch.RepositoryStore().(*gitrepository.GitResourceRepository); !ok {
			t.Fatalf("expected GitResourceRepository, got %T", orch.RepositoryStore())
		}
		if _, ok := orch.ManagedServerClient().(*httpmanagedserver.Client); !ok {
			t.Fatalf("expected Client, got %T", orch.ManagedServerClient())
		}
		if _, ok := orch.SecretProvider().(*filesecrets.FileSecretService); !ok {
			t.Fatalf("expected FileSecretService, got %T", orch.SecretProvider())
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

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "fs-vault-secret"})
		if err != nil {
			t.Fatalf("buildOrchestrator returned error: %v", err)
		}

		if _, ok := orch.RepositoryStore().(*fsstore.LocalResourceRepository); !ok {
			t.Fatalf("expected LocalResourceRepository, got %T", orch.RepositoryStore())
		}
		if _, ok := orch.SecretProvider().(*vaultsecrets.VaultSecretService); !ok {
			t.Fatalf("expected VaultSecretService, got %T", orch.SecretProvider())
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

func TestEmitSecurityWarnings(t *testing.T) {
	t.Parallel()

	t.Run("no_warnings_for_secure_context", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarnings(&buf, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{
					BaseURL: "https://example.com",
				},
			},
			SecretStore: &config.SecretStore{
				Vault: &config.VaultSecretStore{
					Address: "https://vault.example.com",
				},
			},
		})
		if buf.Len() != 0 {
			t.Fatalf("expected no warnings, got %q", buf.String())
		}
	})

	t.Run("warns_on_plain_http_managed_server_base_url", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{
					BaseURL: "http://example.com/api",
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("managed-server.http.base-url uses plain HTTP")) {
			t.Fatalf("expected HTTP warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_managed_server_insecure_skip_verify", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{
					BaseURL: "https://example.com",
					TLS:     &config.TLS{InsecureSkipVerify: true},
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("managed-server.http.tls.insecure-skip-verify")) {
			t.Fatalf("expected TLS skip-verify warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_plain_http_oauth2_token_url", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{
					BaseURL: "https://example.com",
					Auth: &config.HTTPAuth{
						OAuth2: &config.OAuth2{
							TokenURL: "http://auth.local/oauth/token",
						},
					},
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("managed-server.http.auth.oauth2.token-url uses plain HTTP")) {
			t.Fatalf("expected oauth2 token URL warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_plain_http_vault_address", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			SecretStore: &config.SecretStore{
				Vault: &config.VaultSecretStore{
					Address: "http://vault.local:8200",
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("secret-store.vault.address uses plain HTTP")) {
			t.Fatalf("expected Vault HTTP warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_vault_insecure_skip_verify", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			SecretStore: &config.SecretStore{
				Vault: &config.VaultSecretStore{
					Address: "https://vault.example.com",
					TLS:     &config.TLS{InsecureSkipVerify: true},
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("secret-store.vault.tls.insecure-skip-verify")) {
			t.Fatalf("expected Vault TLS skip-verify warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_git_remote_insecure_skip_verify", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			Repository: config.Repository{
				Git: &config.GitRepository{
					Remote: &config.GitRemote{
						TLS: &config.TLS{InsecureSkipVerify: true},
					},
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("repository.git.remote.tls.insecure-skip-verify")) {
			t.Fatalf("expected Git TLS skip-verify warning, got %q", buf.String())
		}
	})

	t.Run("warns_on_ssh_insecure_ignore_host_key", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			Repository: config.Repository{
				Git: &config.GitRepository{
					Remote: &config.GitRemote{
						Auth: &config.GitAuth{
							SSH: &config.SSHAuth{InsecureIgnoreHostKey: true},
						},
					},
				},
			},
		})
		if !bytes.Contains(buf.Bytes(), []byte("repository.git.remote.auth.ssh.insecure-ignore-host-key")) {
			t.Fatalf("expected SSH host-key warning, got %q", buf.String())
		}
	})

	t.Run("emits_all_warnings_combined", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, nil, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{
					BaseURL: "http://example.com",
					TLS:     &config.TLS{InsecureSkipVerify: true},
					Auth: &config.HTTPAuth{
						OAuth2: &config.OAuth2{
							TokenURL: "http://auth.local/oauth/token",
						},
					},
				},
			},
			SecretStore: &config.SecretStore{
				Vault: &config.VaultSecretStore{
					Address: "http://vault.local:8200",
					TLS:     &config.TLS{InsecureSkipVerify: true},
				},
			},
			Repository: config.Repository{
				Git: &config.GitRepository{
					Remote: &config.GitRemote{
						TLS: &config.TLS{InsecureSkipVerify: true},
						Auth: &config.GitAuth{
							SSH: &config.SSHAuth{InsecureIgnoreHostKey: true},
						},
					},
				},
			},
		})

		output := buf.String()
		expectedSubstrings := []string{
			"managed-server.http.base-url uses plain HTTP",
			"managed-server.http.auth.oauth2.token-url uses plain HTTP",
			"managed-server.http.tls.insecure-skip-verify",
			"secret-store.vault.address uses plain HTTP",
			"secret-store.vault.tls.insecure-skip-verify",
			"repository.git.remote.tls.insecure-skip-verify",
			"repository.git.remote.auth.ssh.insecure-ignore-host-key",
		}
		for _, substr := range expectedSubstrings {
			if !bytes.Contains([]byte(output), []byte(substr)) {
				t.Errorf("expected warning containing %q in output:\n%s", substr, output)
			}
		}
		if got, want := strings.Count(output, "[WARNING]"), len(expectedSubstrings); got != want {
			t.Fatalf("expected %d warning labels, got %d in output:\n%s", want, got, output)
		}
	})

	t.Run("ignore_warnings_suppresses_output", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarningsWithArgs(&buf, []string{"--ignore-warnings"}, config.Context{
			ManagedServer: &config.ManagedServer{
				HTTP: &config.HTTPServer{BaseURL: "http://example.com"},
			},
		})
		if buf.Len() != 0 {
			t.Fatalf("expected warnings to be suppressed, got %q", buf.String())
		}
	})

	t.Run("no_warnings_for_empty_context", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		emitSecurityWarnings(&buf, config.Context{})
		if buf.Len() != 0 {
			t.Fatalf("expected no warnings for empty context, got %q", buf.String())
		}
	})
}

func TestBuildOrchestratorValidationAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil_context_service", func(t *testing.T) {
		t.Parallel()

		_, err := buildOrchestrator(context.Background(), nil, config.ContextSelection{})
		if err == nil {
			t.Fatal("expected error")
		}

		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("context_service_error_is_propagated", func(t *testing.T) {
		t.Parallel()

		expected := faults.NewTypedError(faults.NotFoundError, "context not found", nil)
		contextService := &fakeContextService{resolveErr: expected}

		_, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "missing"})
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

		orch, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid"})
		if err != nil {
			t.Fatalf("expected missing repository to be allowed, got error: %v", err)
		}
		if orch.RepositoryStore() != nil {
			t.Fatalf("expected nil repository manager, got %T", orch.RepositoryStore())
		}
	})

	t.Run("invalid_managed_server_provider_configuration", func(t *testing.T) {
		t.Parallel()

		contextService := &fakeContextService{
			resolvedContext: config.Context{
				Name: "invalid-managed-server",
				Repository: config.Repository{
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				ManagedServer: &config.ManagedServer{
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

		_, err := buildOrchestrator(context.Background(), contextService, config.ContextSelection{Name: "invalid-managed-server"})
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
