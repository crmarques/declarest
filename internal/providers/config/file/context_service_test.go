package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
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

	catalog := config.ContextCatalog{
		Contexts:   []config.Context{{Name: "dev", Repository: validFilesystemRepository()}},
		CurrentCtx: "prod",
	}

	err := validateCatalog(catalog)
	if err == nil {
		t.Fatal("expected current-ctx mismatch error")
	}
}

func TestValidateCatalogDuplicateContextNames(t *testing.T) {
	t.Parallel()

	catalog := config.ContextCatalog{
		Contexts: []config.Context{
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
		cfg  config.Context
	}{
		{
			name: "repository_multiple_backends",
			cfg: config.Context{
				Name: "dev",
				Repository: config.Repository{
					Git:        &config.GitRepository{Local: config.GitLocal{BaseDir: "/tmp/repo"}},
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
		},
		{
			name: "managed_server_no_auth",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{BaseURL: "https://example.com"},
				},
			},
		},
		{
			name: "secret_store_multiple_backends",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				SecretStore: &config.SecretStore{
					File:  &config.FileSecretStore{Path: "/tmp/secrets.json", Passphrase: "secret"},
					Vault: &config.VaultSecretStore{Address: "https://vault.example.com", Auth: &config.VaultAuth{Token: "x"}},
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

	resolvedDefault, err := resolveCatalogPath(config.DefaultCatalogPath)
	if err != nil {
		t.Fatalf("resolveCatalogPath default failed: %v", err)
	}

	expectedDefault := filepath.Join(home, "declarest/config/contexts.yaml")
	if resolvedDefault != expectedDefault {
		t.Fatalf("expected %q, got %q", expectedDefault, resolvedDefault)
	}

	envPath := filepath.Join(t.TempDir(), "contexts.yaml")
	t.Setenv(config.ContextFileEnvVar, envPath)
	resolvedFromEnv, err := resolveCatalogPath("")
	if err != nil {
		t.Fatalf("resolveCatalogPath env failed: %v", err)
	}
	if resolvedFromEnv != envPath {
		t.Fatalf("expected env path %q, got %q", envPath, resolvedFromEnv)
	}
}

func TestResolveContextUnknownOverrideFails(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(validCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test catalog: %v", err)
	}

	contextService := NewFileContextService(path)
	_, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name:      "dev",
		Overrides: map[string]string{"unknown.key": "value"},
	})
	if err == nil {
		t.Fatal("expected unknown override error")
	}
	if !strings.Contains(err.Error(), "unknown override key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveContextSelectionAndPrecedence(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test catalog: %v", err)
	}

	contextService := NewFileContextService(path)

	t.Run("explicit_context_selected", func(t *testing.T) {
		t.Parallel()

		resolvedContext, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "git"})
		if err != nil {
			t.Fatalf("ResolveContext returned error: %v", err)
		}
		if resolvedContext.Name != "git" {
			t.Fatalf("expected resolved context name git, got %q", resolvedContext.Name)
		}
		if resolvedContext.Repository.Git == nil {
			t.Fatal("expected git repository to be configured")
		}
	})

	t.Run("empty_name_uses_current_context", func(t *testing.T) {
		t.Parallel()

		resolvedContext, err := contextService.ResolveContext(context.Background(), config.ContextSelection{})
		if err != nil {
			t.Fatalf("ResolveContext returned error: %v", err)
		}
		if resolvedContext.Name != "fs" {
			t.Fatalf("expected current context fs, got %q", resolvedContext.Name)
		}
	})

	t.Run("unknown_context_returns_not_found", func(t *testing.T) {
		t.Parallel()

		_, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "missing"})
		if err == nil {
			t.Fatal("expected unknown context to fail")
		}
		if !strings.Contains(err.Error(), "context \"missing\" not found") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("runtime_override_takes_precedence", func(t *testing.T) {
		t.Parallel()

		resolvedContext, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
			Name:      "fs",
			Overrides: map[string]string{"repository.filesystem.base-dir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("ResolveContext returned error: %v", err)
		}
		if resolvedContext.Repository.Filesystem == nil {
			t.Fatal("expected filesystem repository to be configured")
		}
		if resolvedContext.Repository.Filesystem.BaseDir != "/tmp/override" {
			t.Fatalf("expected override base-dir /tmp/override, got %q", resolvedContext.Repository.Filesystem.BaseDir)
		}
	})
}

func validFilesystemRepository() config.Repository {
	return config.Repository{
		ResourceFormat: config.ResourceFormatJSON,
		Filesystem:     &config.FilesystemRepository{BaseDir: "/tmp/repo"},
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

const providerSelectionCatalogYAML = `
contexts:
  - name: fs
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo

  - name: git
    repository:
      resource-format: json
      git:
        local:
          base-dir: /tmp/repo

  - name: http
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

  - name: file-secret
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    secret-store:
      file:
        path: /tmp/secrets.json
        passphrase: change-me

  - name: vault-secret
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    secret-store:
      vault:
        address: https://vault.example.com
        auth:
          token: s.xxxx

current-ctx: fs
`
