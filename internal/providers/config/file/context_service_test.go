package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestDecodeCatalogSuccess(t *testing.T) {
	t.Parallel()

	contextCatalog, err := decodeCatalog([]byte(validContextCatalogYAML))
	if err != nil {
		t.Fatalf("decodeCatalog returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contextCatalog.Contexts))
	}
	if contextCatalog.CurrentCtx != "dev" {
		t.Fatalf("expected currentCtx dev, got %q", contextCatalog.CurrentCtx)
	}
}

func TestDecodeCatalogGitLocalAutoInitDefaultsTrueWhenOmitted(t *testing.T) {
	t.Parallel()

	contextCatalog, err := decodeCatalog([]byte(`
contexts:
  - name: git
    repository:
      git:
        local:
          baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
currentCtx: git
`))
	if err != nil {
		t.Fatalf("decodeCatalog returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 || contextCatalog.Contexts[0].Repository.Git == nil {
		t.Fatalf("expected one git context, got %#v", contextCatalog.Contexts)
	}

	local := contextCatalog.Contexts[0].Repository.Git.Local
	if !local.AutoInitEnabled() {
		t.Fatal("expected repository.git.local.autoInit to default to true when omitted")
	}
	if local.AutoInit != nil {
		t.Fatalf("expected omitted autoInit to remain nil for compact persistence, got %#v", local.AutoInit)
	}
}

func TestDecodeCatalogGitLocalAutoInitHonorsFalse(t *testing.T) {
	t.Parallel()

	contextCatalog, err := decodeCatalog([]byte(`
contexts:
  - name: git
    repository:
      git:
        local:
          baseDir: /tmp/repo
          autoInit: false
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
currentCtx: git
`))
	if err != nil {
		t.Fatalf("decodeCatalog returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 || contextCatalog.Contexts[0].Repository.Git == nil {
		t.Fatalf("expected one git context, got %#v", contextCatalog.Contexts)
	}

	local := contextCatalog.Contexts[0].Repository.Git.Local
	if local.AutoInit == nil {
		t.Fatal("expected explicit autoInit=false to be preserved")
	}
	if local.AutoInitEnabled() {
		t.Fatal("expected repository.git.local.autoInit=false to be respected")
	}
}

func TestDecodeCatalogRejectsUnknownField(t *testing.T) {
	t.Parallel()

	invalidYAML := `
contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/repo
        unknown-key: true
currentCtx: dev
`
	_, err := decodeCatalog([]byte(invalidYAML))
	if err == nil {
		t.Fatal("expected unknown field to fail decode")
	}
}

func TestValidateCatalogCurrentContextMissing(t *testing.T) {
	t.Parallel()

	contextCatalog := config.ContextCatalog{
		Contexts: []config.Context{{
			Name:          "dev",
			Repository:    validFilesystemRepository(),
			ManagedServer: validManagedServer(),
		}},
		CurrentCtx: "prod",
	}

	err := validateCatalog(contextCatalog)
	if err == nil {
		t.Fatal("expected currentCtx mismatch error")
	}
}

func TestValidateCatalogDuplicateContextNames(t *testing.T) {
	t.Parallel()

	contextCatalog := config.ContextCatalog{
		Contexts: []config.Context{
			{Name: "dev", Repository: validFilesystemRepository(), ManagedServer: validManagedServer()},
			{Name: "dev", Repository: validFilesystemRepository(), ManagedServer: validManagedServer()},
		},
		CurrentCtx: "dev",
	}

	err := validateCatalog(contextCatalog)
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
				Name:          "dev",
				ManagedServer: validManagedServer(),
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
			name: "managed_server_missing",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
			},
		},
		{
			name: "managed_server_proxy_auth_incomplete",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL: "https://example.com",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
						Proxy: &config.HTTPProxy{
							HTTPURL: "http://proxy.example.com:3128",
							Auth: &config.ProxyAuth{
								Username: "user",
							},
						},
					},
				},
			},
		},
		{
			name: "managed_server_health_check_query_not_supported",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				ManagedServer: &config.ManagedServer{
					HTTP: &config.HTTPServer{
						BaseURL:     "https://example.com",
						HealthCheck: "https://example.com/health?probe=true",
						Auth: &config.HTTPAuth{
							CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
						},
					},
				},
			},
		},
		{
			name: "secret_store_multiple_backends",
			cfg: config.Context{
				Name:          "dev",
				Repository:    validFilesystemRepository(),
				ManagedServer: validManagedServer(),
				SecretStore: &config.SecretStore{
					File:  &config.FileSecretStore{Path: "/tmp/secrets.json", Passphrase: "secret"},
					Vault: &config.VaultSecretStore{Address: "https://vault.example.com", Auth: &config.VaultAuth{Token: "x"}},
				},
			},
		},
		{
			name: "metadata_multiple_sources",
			cfg: config.Context{
				Name:          "dev",
				Repository:    validFilesystemRepository(),
				ManagedServer: validManagedServer(),
				Metadata: config.Metadata{
					BaseDir:    "/tmp/metadata",
					Bundle:     "keycloak-bundle:0.0.1",
					BundleFile: "/tmp/keycloak-bundle-0.0.1.tar.gz",
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

func TestValidateConfigAllowsExplicitProxyDisable(t *testing.T) {
	t.Parallel()

	server := validManagedServer()
	server.HTTP.Proxy = &config.HTTPProxy{}

	err := validateConfig(config.Context{
		Name:          "proxy-disable",
		Repository:    validFilesystemRepository(),
		ManagedServer: server,
	})
	if err != nil {
		t.Fatalf("expected explicit proxy disable to be valid, got %v", err)
	}
}

func TestValidateConfigAllowsMissingRepositoryWhenManagedServerIsConfigured(t *testing.T) {
	t.Parallel()

	err := validateConfig(config.Context{
		Name:          "remote-only",
		ManagedServer: validManagedServer(),
	})
	if err != nil {
		t.Fatalf("expected repository to be optional, got error: %v", err)
	}
}

func TestResolveCatalogPathDefaultAndEnv(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to resolve home dir: %v", err)
	}

	resolvedDefault, err := resolveCatalogPath(config.DefaultContextCatalogPath)
	if err != nil {
		t.Fatalf("resolveCatalogPath default failed: %v", err)
	}

	expectedDefault := filepath.Join(home, ".declarest/configs/contexts.yaml")
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
	if err := os.WriteFile(path, []byte(validContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
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
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
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
			Overrides: map[string]string{"repository.filesystem.baseDir": "/tmp/override"},
		})
		if err != nil {
			t.Fatalf("ResolveContext returned error: %v", err)
		}
		if resolvedContext.Repository.Filesystem == nil {
			t.Fatal("expected filesystem repository to be configured")
		}
		if resolvedContext.Repository.Filesystem.BaseDir != "/tmp/override" {
			t.Fatalf("expected override baseDir /tmp/override, got %q", resolvedContext.Repository.Filesystem.BaseDir)
		}
	})
}

func TestFileContextServiceCreateWritesUserOnlyCatalogPermissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode semantics are not portable on Windows")
	}

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	err := contextService.Create(context.Background(), config.Context{
		Name:          "dev",
		Repository:    validFilesystemRepository(),
		ManagedServer: validManagedServer(),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat catalog: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected 0600 permissions, got %#o", got)
	}
}

func TestFileContextServiceLoadCatalogNormalizesPermissiveFileMode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode semantics are not portable on Windows")
	}

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(validContextCatalogYAML), 0o644); err != nil {
		t.Fatalf("failed to write test catalog: %v", err)
	}

	contextService := NewFileContextService(path)
	if _, err := contextService.List(context.Background()); err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat catalog: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected normalized 0600 permissions, got %#o", got)
	}
}

func TestContextServiceMissingCatalogBehaviors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	items, err := contextService.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}

	_, err = contextService.GetCurrent(context.Background())
	assertTypedCategory(t, err, faults.NotFoundError)
	if !strings.Contains(err.Error(), "current context not set") {
		t.Fatalf("unexpected get current error: %v", err)
	}

	_, err = contextService.ResolveContext(context.Background(), config.ContextSelection{})
	assertTypedCategory(t, err, faults.NotFoundError)
	if !strings.Contains(err.Error(), "current context not set") {
		t.Fatalf("unexpected resolve error: %v", err)
	}

	if err := contextService.SetCurrent(context.Background(), "missing"); err == nil {
		t.Fatal("expected SetCurrent on empty contextCatalog to fail")
	} else {
		assertTypedCategory(t, err, faults.NotFoundError)
	}
}

func TestContextServiceCRUDLifecycle(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	dev := config.Context{
		Name:          "dev",
		ManagedServer: validManagedServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/dev"},
		},
	}
	if err := contextService.Create(context.Background(), dev); err != nil {
		t.Fatalf("Create(dev) returned error: %v", err)
	}

	prod := config.Context{
		Name:          "prod",
		ManagedServer: validManagedServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/prod"},
		},
	}
	if err := contextService.Create(context.Background(), prod); err != nil {
		t.Fatalf("Create(prod) returned error: %v", err)
	}

	current, err := contextService.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("GetCurrent returned error: %v", err)
	}
	if current.Name != "dev" {
		t.Fatalf("expected current context dev, got %q", current.Name)
	}

	if err := contextService.SetCurrent(context.Background(), "prod"); err != nil {
		t.Fatalf("SetCurrent(prod) returned error: %v", err)
	}

	current, err = contextService.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("GetCurrent after SetCurrent returned error: %v", err)
	}
	if current.Name != "prod" {
		t.Fatalf("expected current context prod, got %q", current.Name)
	}

	if err := contextService.Rename(context.Background(), "prod", "stage"); err != nil {
		t.Fatalf("Rename(prod->stage) returned error: %v", err)
	}

	current, err = contextService.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("GetCurrent after Rename returned error: %v", err)
	}
	if current.Name != "stage" {
		t.Fatalf("expected current context stage after rename, got %q", current.Name)
	}

	if err := contextService.Update(context.Background(), config.Context{
		Name:          "stage",
		ManagedServer: validManagedServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/stage"},
		},
	}); err != nil {
		t.Fatalf("Update(stage) returned error: %v", err)
	}

	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "stage"})
	if err != nil {
		t.Fatalf("ResolveContext(stage) returned error: %v", err)
	}
	if resolved.Repository.Filesystem == nil || resolved.Repository.Filesystem.BaseDir != "/tmp/stage" {
		t.Fatalf("expected updated filesystem baseDir, got %#v", resolved.Repository.Filesystem)
	}

	if err := contextService.Delete(context.Background(), "stage"); err != nil {
		t.Fatalf("Delete(stage) returned error: %v", err)
	}

	current, err = contextService.GetCurrent(context.Background())
	if err != nil {
		t.Fatalf("GetCurrent after deleting current context returned error: %v", err)
	}
	if current.Name != "dev" {
		t.Fatalf("expected fallback current context dev, got %q", current.Name)
	}

	if err := contextService.Delete(context.Background(), "dev"); err != nil {
		t.Fatalf("Delete(dev) returned error: %v", err)
	}

	items, err := contextService.List(context.Background())
	if err != nil {
		t.Fatalf("List after deleting all contexts returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty contextCatalog, got %#v", items)
	}

	if _, err := contextService.GetCurrent(context.Background()); err == nil {
		t.Fatal("expected GetCurrent to fail when contextCatalog is empty")
	} else {
		assertTypedCategory(t, err, faults.NotFoundError)
	}
}

func TestSetCurrentPreservesContextOrder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	for _, name := range []string{"a", "b", "c"} {
		if err := contextService.Create(context.Background(), config.Context{
			Name:          name,
			ManagedServer: validManagedServer(),
			Repository: config.Repository{
				Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/" + name},
			},
		}); err != nil {
			t.Fatalf("Create(%q) returned error: %v", name, err)
		}
	}

	if err := contextService.SetCurrent(context.Background(), "b"); err != nil {
		t.Fatalf("SetCurrent(b) returned error: %v", err)
	}

	items, err := contextService.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(items))
	}
	if items[0].Name != "a" || items[1].Name != "b" || items[2].Name != "c" {
		t.Fatalf("expected preserved order [a b c], got [%s %s %s]", items[0].Name, items[1].Name, items[2].Name)
	}
}

func TestCreateDoesNotPersistRemovedRepositoryResourceFormat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	if err := contextService.Create(context.Background(), config.Context{
		Name:          "dev",
		ManagedServer: validManagedServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
		},
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	contextCatalog, err := decodeCatalogFile(path)
	if err != nil {
		t.Fatalf("decodeCatalogFile returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(contextCatalog.Contexts))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved context catalog: %v", err)
	}
	if strings.Contains(string(raw), "resourceFormat:") {
		t.Fatalf("expected resourceFormat to be omitted, got:\n%s", string(raw))
	}
}

func TestMetadataBaseDirMatchingRepositoryIsNotPersisted(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	if err := contextService.Create(context.Background(), config.Context{
		Name:          "dev",
		ManagedServer: validManagedServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
		},
		Metadata: config.Metadata{
			BaseDir: "/tmp/repo",
		},
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved context catalog: %v", err)
	}
	if strings.Contains(string(raw), "metadata:") {
		t.Fatalf("expected metadata block to be omitted when baseDir matches repository baseDir, got:\n%s", string(raw))
	}

	contextCatalog, err := decodeCatalogFile(path)
	if err != nil {
		t.Fatalf("decodeCatalogFile returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(contextCatalog.Contexts))
	}
	if contextCatalog.Contexts[0].Metadata.BaseDir != "" {
		t.Fatalf("expected persisted metadata baseDir to be empty, got %q", contextCatalog.Contexts[0].Metadata.BaseDir)
	}
}

func TestResolveContextWithoutRepositoryResourceFormat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(contextCatalogWithoutResourceFormatYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "dev"})
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	if resolved.Repository.Filesystem == nil || resolved.Repository.Filesystem.BaseDir != "/tmp/repo" {
		t.Fatalf("expected filesystem repository to resolve, got %#v", resolved.Repository.Filesystem)
	}
}

func TestResolveContextDefaultsMetadataBaseDirWhenMissing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(contextCatalogWithoutResourceFormatYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "dev"})
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	if resolved.Metadata.BaseDir != "/tmp/repo" {
		t.Fatalf("expected metadata baseDir default /tmp/repo, got %q", resolved.Metadata.BaseDir)
	}
}

func TestResolveContextDoesNotDefaultMetadataBaseDirWhenBundleIsConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(`
contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    metadata:
      bundle: keycloak-bundle:0.0.1
currentCtx: dev
`), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "dev"})
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	if resolved.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata baseDir to stay empty for bundle contexts, got %q", resolved.Metadata.BaseDir)
	}
	if resolved.Metadata.Bundle != "keycloak-bundle:0.0.1" {
		t.Fatalf("expected metadata bundle keycloak-bundle:0.0.1, got %q", resolved.Metadata.Bundle)
	}
}

func TestResolveContextDoesNotDefaultMetadataBaseDirWhenBundleFileIsConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(`
contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    metadata:
      bundleFile: /tmp/keycloak-bundle-0.0.1.tar.gz
currentCtx: dev
`), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "dev"})
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	if resolved.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata baseDir to stay empty for bundleFile contexts, got %q", resolved.Metadata.BaseDir)
	}
	if resolved.Metadata.BundleFile != "/tmp/keycloak-bundle-0.0.1.tar.gz" {
		t.Fatalf("expected metadata bundleFile /tmp/keycloak-bundle-0.0.1.tar.gz, got %q", resolved.Metadata.BundleFile)
	}
}

func TestResolveContextOverrideSupportsMetadataBundle(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name: "fs",
		Overrides: map[string]string{
			"metadata.bundle": "keycloak-bundle:0.0.1",
		},
	})
	if err != nil {
		t.Fatalf("expected metadata bundle override to succeed, got %v", err)
	}
	if resolved.Metadata.Bundle != "keycloak-bundle:0.0.1" {
		t.Fatalf("expected metadata bundle override keycloak-bundle:0.0.1, got %q", resolved.Metadata.Bundle)
	}
	if resolved.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata baseDir to be cleared when bundle override is configured, got %q", resolved.Metadata.BaseDir)
	}
	if resolved.Metadata.BundleFile != "" {
		t.Fatalf("expected metadata bundleFile to be cleared when bundle override is configured, got %q", resolved.Metadata.BundleFile)
	}
}

func TestResolveContextOverrideSupportsMetadataBundleFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name: "fs",
		Overrides: map[string]string{
			"metadata.bundleFile": "/tmp/keycloak-bundle-0.0.1.tar.gz",
		},
	})
	if err != nil {
		t.Fatalf("expected metadata bundleFile override to succeed, got %v", err)
	}
	if resolved.Metadata.BundleFile != "/tmp/keycloak-bundle-0.0.1.tar.gz" {
		t.Fatalf("expected metadata bundleFile override /tmp/keycloak-bundle-0.0.1.tar.gz, got %q", resolved.Metadata.BundleFile)
	}
	if resolved.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata baseDir to be cleared when bundleFile override is configured, got %q", resolved.Metadata.BaseDir)
	}
	if resolved.Metadata.Bundle != "" {
		t.Fatalf("expected metadata bundle to be cleared when bundleFile override is configured, got %q", resolved.Metadata.Bundle)
	}
}

func TestResolveContextOverrideSupportsManagedServerWhenConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name:      "fs",
		Overrides: map[string]string{"managedServer.http.baseUrl": "https://override.example.com"},
	})
	if err != nil {
		t.Fatalf("expected managedServer override to succeed, got %v", err)
	}
	if resolved.ManagedServer == nil || resolved.ManagedServer.HTTP == nil {
		t.Fatalf("expected managedServer configuration, got %#v", resolved.ManagedServer)
	}
	if resolved.ManagedServer.HTTP.BaseURL != "https://override.example.com" {
		t.Fatalf("expected managedServer baseUrl override, got %q", resolved.ManagedServer.HTTP.BaseURL)
	}
}

func TestResolveContextOverrideSupportsManagedServerHealthCheckWhenConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name:      "fs",
		Overrides: map[string]string{"managedServer.http.healthCheck": "https://override.example.com/healthz"},
	})
	if err != nil {
		t.Fatalf("expected managedServer healthCheck override to succeed, got %v", err)
	}
	if resolved.ManagedServer == nil || resolved.ManagedServer.HTTP == nil {
		t.Fatalf("expected managedServer configuration, got %#v", resolved.ManagedServer)
	}
	if resolved.ManagedServer.HTTP.HealthCheck != "https://override.example.com/healthz" {
		t.Fatalf("expected managedServer healthCheck override, got %q", resolved.ManagedServer.HTTP.HealthCheck)
	}
}

func TestResolveContextOverrideSupportsManagedServerProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name: "fs",
		Overrides: map[string]string{
			"managedServer.http.proxy.httpUrl":       "http://proxy.example.com:3128",
			"managedServer.http.proxy.httpsUrl":      "https://proxy.example.com:3128",
			"managedServer.http.proxy.noProxy":       "localhost,127.0.0.1",
			"managedServer.http.proxy.auth.username": "proxy-user",
			"managedServer.http.proxy.auth.password": "proxy-pass",
		},
	})
	if err != nil {
		t.Fatalf("expected managedServer proxy overrides to succeed, got %v", err)
	}

	if resolved.ManagedServer == nil || resolved.ManagedServer.HTTP == nil || resolved.ManagedServer.HTTP.Proxy == nil {
		t.Fatalf("expected managedServer proxy configuration, got %#v", resolved.ManagedServer)
	}
	if resolved.ManagedServer.HTTP.Proxy.HTTPURL != "http://proxy.example.com:3128" {
		t.Fatalf("expected proxy httpUrl override, got %q", resolved.ManagedServer.HTTP.Proxy.HTTPURL)
	}
	if resolved.ManagedServer.HTTP.Proxy.HTTPSURL != "https://proxy.example.com:3128" {
		t.Fatalf("expected proxy httpsUrl override, got %q", resolved.ManagedServer.HTTP.Proxy.HTTPSURL)
	}
	if resolved.ManagedServer.HTTP.Proxy.NoProxy != "localhost,127.0.0.1" {
		t.Fatalf("expected proxy noProxy override, got %q", resolved.ManagedServer.HTTP.Proxy.NoProxy)
	}
	if resolved.ManagedServer.HTTP.Proxy.Auth == nil {
		t.Fatal("expected proxy auth configuration")
	}
	if resolved.ManagedServer.HTTP.Proxy.Auth.Username != "proxy-user" {
		t.Fatalf("expected proxy auth username override, got %q", resolved.ManagedServer.HTTP.Proxy.Auth.Username)
	}
	if resolved.ManagedServer.HTTP.Proxy.Auth.Password != "proxy-pass" {
		t.Fatalf("expected proxy auth password override, got %q", resolved.ManagedServer.HTTP.Proxy.Auth.Password)
	}
}

func TestResolveContextProxyInheritance(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(proxyInheritanceContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write proxy inheritance context catalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "shared"})
	if err != nil {
		t.Fatalf("expected proxy inheritance to succeed, got %v", err)
	}

	assertProxyConfig(t, "managedServer", resolved.ManagedServer.HTTP.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
	assertProxyConfig(t, "repository", resolved.Repository.Git.Remote.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
	assertProxyConfig(t, "secretStore", resolved.SecretStore.Vault.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
	assertProxyConfig(t, "metadata", resolved.Metadata.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
}

func TestResolveContextProxyExplicitDisable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(proxyDisableContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write proxy disable context catalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "disable"})
	if err != nil {
		t.Fatalf("expected proxy disable scenario to succeed, got %v", err)
	}

	assertProxyConfig(t, "managedServer", resolved.ManagedServer.HTTP.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
	assertProxyConfig(t, "repository", resolved.Repository.Git.Remote.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")
	assertProxyConfig(t, "secretStore", resolved.SecretStore.Vault.Proxy, "http://proxy.example.com:3128", "https://proxy.example.com:3128", "localhost,127.0.0.1", "proxy-user", "proxy-pass")

	if resolved.Metadata.Proxy == nil {
		t.Fatalf("expected metadata proxy block to remain present even when disabling, got nil")
	}
	if resolved.Metadata.Proxy.HTTPURL != "" || resolved.Metadata.Proxy.HTTPSURL != "" || resolved.Metadata.Proxy.NoProxy != "" {
		t.Fatalf("expected metadata proxy to remain empty when explicitly disabled, got %#v", resolved.Metadata.Proxy)
	}
	if resolved.Metadata.Proxy.Auth != nil {
		t.Fatalf("expected metadata proxy auth to remain empty when explicitly disabled, got %#v", resolved.Metadata.Proxy.Auth)
	}
}

func TestUpdatePreservesProxyOmissionsFromStoredContext(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(proxyPersistenceContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write proxy persistence context catalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{Name: "persist"})
	if err != nil {
		t.Fatalf("expected resolve to succeed, got %v", err)
	}

	if resolved.Repository.Git == nil || resolved.Repository.Git.Remote == nil || resolved.Repository.Git.Remote.Proxy == nil {
		t.Fatalf("expected resolved repository proxy to be inherited, got %#v", resolved.Repository.Git)
	}
	if resolved.SecretStore == nil || resolved.SecretStore.Vault == nil || resolved.SecretStore.Vault.Proxy == nil {
		t.Fatalf("expected resolved vault proxy to be inherited, got %#v", resolved.SecretStore)
	}
	if resolved.Metadata.Proxy == nil {
		t.Fatalf("expected resolved metadata proxy to be inherited, got %#v", resolved.Metadata)
	}

	if err := contextService.Update(context.Background(), resolved); err != nil {
		t.Fatalf("expected update with resolved context to succeed, got %v", err)
	}

	contextCatalog, err := decodeCatalogFile(path)
	if err != nil {
		t.Fatalf("failed to decode persisted context catalog: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(contextCatalog.Contexts))
	}
	persisted := contextCatalog.Contexts[0]
	if persisted.ManagedServer == nil || persisted.ManagedServer.HTTP == nil || persisted.ManagedServer.HTTP.Proxy == nil {
		t.Fatalf("expected managedServer proxy to remain persisted, got %#v", persisted.ManagedServer)
	}
	if persisted.Repository.Git == nil || persisted.Repository.Git.Remote == nil {
		t.Fatalf("expected git repository to remain persisted, got %#v", persisted.Repository.Git)
	}
	if persisted.Repository.Git.Remote.Proxy != nil {
		t.Fatalf("expected repository proxy omission to be preserved, got %#v", persisted.Repository.Git.Remote.Proxy)
	}
	if persisted.SecretStore == nil || persisted.SecretStore.Vault == nil {
		t.Fatalf("expected vault configuration to remain persisted, got %#v", persisted.SecretStore)
	}
	if persisted.SecretStore.Vault.Proxy != nil {
		t.Fatalf("expected vault proxy omission to be preserved, got %#v", persisted.SecretStore.Vault.Proxy)
	}
	if persisted.Metadata.Proxy != nil {
		t.Fatalf("expected metadata proxy omission to be preserved, got %#v", persisted.Metadata.Proxy)
	}
}

func assertProxyConfig(t *testing.T, component string, proxy *config.HTTPProxy, httpURL, httpsURL, noProxy, username, password string) {
	t.Helper()
	if proxy == nil {
		t.Fatalf("expected %s proxy to be configured, got nil", component)
	}
	if proxy.HTTPURL != httpURL {
		t.Fatalf("expected %s proxy httpUrl %q, got %q", component, httpURL, proxy.HTTPURL)
	}
	if proxy.HTTPSURL != httpsURL {
		t.Fatalf("expected %s proxy httpsUrl %q, got %q", component, httpsURL, proxy.HTTPSURL)
	}
	if proxy.NoProxy != noProxy {
		t.Fatalf("expected %s proxy noProxy %q, got %q", component, noProxy, proxy.NoProxy)
	}
	if proxy.Auth == nil {
		t.Fatalf("expected %s proxy auth to be configured", component)
	}
	if proxy.Auth.Username != username {
		t.Fatalf("expected %s proxy auth username %q, got %q", component, username, proxy.Auth.Username)
	}
	if proxy.Auth.Password != password {
		t.Fatalf("expected %s proxy auth password %q, got %q", component, password, proxy.Auth.Password)
	}
}

func TestResolveContextOverrideFailureIsDeterministic(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	_, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name: "fs",
		Overrides: map[string]string{
			"repository.git.local.baseDir": "/tmp/git",
			"aaa.unknown":                  "x",
		},
	})
	if err == nil {
		t.Fatal("expected invalid overrides to fail")
	}
	if !strings.Contains(err.Error(), "unknown override key \"aaa.unknown\"") {
		t.Fatalf("expected deterministic failure on alphabetically first invalid key, got: %v", err)
	}
}

func TestMutationOnMissingCatalogReturnsNotFound(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "update",
			run: func() error {
				return contextService.Update(context.Background(), config.Context{
					Name:          "missing",
					ManagedServer: validManagedServer(),
					Repository: config.Repository{
						Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
					},
				})
			},
		},
		{
			name: "delete",
			run: func() error {
				return contextService.Delete(context.Background(), "missing")
			},
		},
		{
			name: "rename",
			run: func() error {
				return contextService.Rename(context.Background(), "missing", "renamed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			assertTypedCategory(t, err, faults.NotFoundError)
		})
	}
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}

func validFilesystemRepository() config.Repository {
	return config.Repository{
		Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
	}
}

func validManagedServer() *config.ManagedServer {
	return &config.ManagedServer{
		HTTP: &config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "secret-token"}},
			},
		},
	}
}

const validContextCatalogYAML = `
contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    secretStore:
      file:
        path: /tmp/secrets.json
        passphrase: change-me
    metadata:
      baseDir: /tmp/metadata
currentCtx: dev
`

const providerSelectionContextCatalogYAML = `
contexts:
  - name: fs
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token

  - name: git
    repository:
      git:
        local:
          baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token

  - name: http
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token

  - name: file-secret
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    secretStore:
      file:
        path: /tmp/secrets.json
        passphrase: change-me

  - name: vault-secret
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          token: s.xxxx

currentCtx: fs
`

const contextCatalogWithoutResourceFormatYAML = `
contexts:
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/repo
    managedServer:
      http:
        baseUrl: https://example.com/api
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
currentCtx: dev
`

const proxyInheritanceContextCatalogYAML = `
contexts:
  - name: shared
    repository:
      git:
        local:
          baseDir: /tmp/repo
        remote:
          url: https://example.com/config.git
          auth:
            basicAuth:
              username: git
              password: secret
    managedServer:
      http:
        baseUrl: https://api.example.com
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          token: vault-token
        proxy:
          httpUrl: http://proxy.example.com:3128
          httpsUrl: https://proxy.example.com:3128
          noProxy: localhost,127.0.0.1
          auth:
            username: proxy-user
            password: proxy-pass
    metadata:
      baseDir: /tmp/metadata
currentCtx: shared
`

const proxyDisableContextCatalogYAML = `
contexts:
  - name: disable
    repository:
      git:
        local:
          baseDir: /tmp/repo
        remote:
          url: https://example.com/config.git
          auth:
            basicAuth:
              username: git
              password: secret
    managedServer:
      http:
        baseUrl: https://api.example.com
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
        proxy:
          httpUrl: http://proxy.example.com:3128
          httpsUrl: https://proxy.example.com:3128
          noProxy: localhost,127.0.0.1
          auth:
            username: proxy-user
            password: proxy-pass
    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          token: vault-token
    metadata:
      baseDir: /tmp/metadata
      proxy: {}
currentCtx: disable
`

const proxyPersistenceContextCatalogYAML = `
contexts:
  - name: persist
    repository:
      git:
        local:
          baseDir: /tmp/repo
        remote:
          url: https://example.com/config.git
          auth:
            basicAuth:
              username: git
              password: secret
    managedServer:
      http:
        baseUrl: https://api.example.com
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: secret-token
        proxy:
          httpUrl: http://proxy.example.com:3128
          httpsUrl: https://proxy.example.com:3128
          noProxy: localhost,127.0.0.1
          auth:
            username: proxy-user
            password: proxy-pass
    secretStore:
      vault:
        address: https://vault.example.com
        auth:
          token: vault-token
    metadata:
      baseDir: /tmp/metadata
currentCtx: persist
`
