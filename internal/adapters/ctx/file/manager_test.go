package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/ctx"
)

func TestDecodeCatalogSuccess(t *testing.T) {
	t.Parallel()

	catalog, err := decodeCatalog([]byte(validCatalogYAML))
	if err != nil {
		t.Fatalf("decodeCatalog returned error: %v", err)
	}
	if len(catalog.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(catalog.Contexts))
	}
	if catalog.CurrentCtx != "dev" {
		t.Fatalf("expected current-ctx dev, got %q", catalog.CurrentCtx)
	}
}

func TestDecodeCatalogRejectsUnknownField(t *testing.T) {
	t.Parallel()

	invalidYAML := `
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/repo
        unknown-key: true
current-ctx: dev
`
	_, err := decodeCatalog([]byte(invalidYAML))
	if err == nil {
		t.Fatal("expected unknown field to fail decode")
	}
}

func TestValidateCatalogCurrentContextMissing(t *testing.T) {
	t.Parallel()

	catalog := ctx.Catalog{
		Contexts:   []ctx.Config{{Name: "dev", Repository: validFilesystemRepository()}},
		CurrentCtx: "prod",
	}

	err := validateCatalog(catalog)
	if err == nil {
		t.Fatal("expected current-ctx mismatch error")
	}
}

func TestValidateCatalogDuplicateContextNames(t *testing.T) {
	t.Parallel()

	catalog := ctx.Catalog{
		Contexts: []ctx.Config{
			{Name: "dev", Repository: validFilesystemRepository()},
			{Name: "dev", Repository: validFilesystemRepository()},
		},
		CurrentCtx: "dev",
	}

	err := validateCatalog(catalog)
	if err == nil {
		t.Fatal("expected duplicate name validation error")
	}
}

func TestValidateConfigOneOfRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  ctx.Config
	}{
		{
			name: "repository_multiple_backends",
			cfg: ctx.Config{
				Name: "dev",
				Repository: ctx.RepositoryConfig{
					Git:        &ctx.GitRepositoryConfig{Local: ctx.GitLocalConfig{BaseDir: "/tmp/repo"}},
					Filesystem: &ctx.FilesystemRepositoryConfig{BaseDir: "/tmp/repo"},
				},
			},
		},
		{
			name: "managed_server_no_auth",
			cfg: ctx.Config{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				ManagedServer: &ctx.ManagedServerConfig{
					HTTP: &ctx.HTTPServerConfig{BaseURL: "https://example.com"},
				},
			},
		},
		{
			name: "secret_store_multiple_backends",
			cfg: ctx.Config{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				SecretStore: &ctx.SecretStoreConfig{
					File:  &ctx.FileSecretStoreConfig{Path: "/tmp/secrets.json", Passphrase: "secret"},
					Vault: &ctx.VaultSecretStoreConfig{Address: "https://vault.example.com", Auth: &ctx.VaultAuthConfig{Token: "x"}},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateConfig(tt.cfg); err == nil {
				t.Fatalf("expected validation failure for %s", tt.name)
			}
		})
	}
}

func TestResolveCatalogPathDefaultAndEnv(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to resolve home dir: %v", err)
	}

	resolvedDefault, err := resolveCatalogPath(ctx.DefaultCatalogPath)
	if err != nil {
		t.Fatalf("resolveCatalogPath default failed: %v", err)
	}

	expectedDefault := filepath.Join(home, "declarest/config/contexts.yaml")
	if resolvedDefault != expectedDefault {
		t.Fatalf("expected %q, got %q", expectedDefault, resolvedDefault)
	}

	envPath := filepath.Join(t.TempDir(), "contexts.yaml")
	t.Setenv(ctx.ContextFileEnvVar, envPath)
	resolvedFromEnv, err := resolveCatalogPath("")
	if err != nil {
		t.Fatalf("resolveCatalogPath env failed: %v", err)
	}
	if resolvedFromEnv != envPath {
		t.Fatalf("expected env path %q, got %q", envPath, resolvedFromEnv)
	}
}

func TestLoadResolvedConfigUnknownOverrideFails(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(validCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test catalog: %v", err)
	}

	manager := NewManager(path)
	_, err := manager.LoadResolvedConfig(context.Background(), "dev", map[string]string{"unknown.key": "value"})
	if err == nil {
		t.Fatal("expected unknown override error")
	}
	if !strings.Contains(err.Error(), "unknown override key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func validFilesystemRepository() ctx.RepositoryConfig {
	return ctx.RepositoryConfig{
		ResourceFormat: ctx.ResourceFormatJSON,
		Filesystem:     &ctx.FilesystemRepositoryConfig{BaseDir: "/tmp/repo"},
	}
}

const validCatalogYAML = `
contexts:
  - name: dev
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    managed-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token
    secret-store:
      file:
        path: /tmp/secrets.json
        passphrase: change-me
    metadata:
      base-dir: /tmp/metadata
current-ctx: dev
`
