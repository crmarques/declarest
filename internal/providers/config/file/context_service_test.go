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
		t.Fatalf("expected current-ctx dev, got %q", contextCatalog.CurrentCtx)
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

	contextCatalog := config.ContextCatalog{
		Contexts: []config.Context{{
			Name:           "dev",
			Repository:     validFilesystemRepository(),
			ResourceServer: validResourceServer(),
		}},
		CurrentCtx: "prod",
	}

	err := validateCatalog(contextCatalog)
	if err == nil {
		t.Fatal("expected current-ctx mismatch error")
	}
}

func TestValidateCatalogDuplicateContextNames(t *testing.T) {
	t.Parallel()

	contextCatalog := config.ContextCatalog{
		Contexts: []config.Context{
			{Name: "dev", Repository: validFilesystemRepository(), ResourceServer: validResourceServer()},
			{Name: "dev", Repository: validFilesystemRepository(), ResourceServer: validResourceServer()},
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
				Name:           "dev",
				ResourceServer: validResourceServer(),
				Repository: config.Repository{
					Git:        &config.GitRepository{Local: config.GitLocal{BaseDir: "/tmp/repo"}},
					Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
		},
		{
			name: "resource_server_no_auth",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
				ResourceServer: &config.ResourceServer{
					HTTP: &config.HTTPServer{BaseURL: "https://example.com"},
				},
			},
		},
		{
			name: "resource_server_missing",
			cfg: config.Context{
				Name:       "dev",
				Repository: validFilesystemRepository(),
			},
		},
		{
			name: "secret_store_multiple_backends",
			cfg: config.Context{
				Name:           "dev",
				Repository:     validFilesystemRepository(),
				ResourceServer: validResourceServer(),
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

func TestValidateConfigAllowsMissingRepositoryWhenResourceServerIsConfigured(t *testing.T) {
	t.Parallel()

	err := validateConfig(config.Context{
		Name:           "remote-only",
		ResourceServer: validResourceServer(),
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

func TestFileContextServiceCreateWritesUserOnlyCatalogPermissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode semantics are not portable on Windows")
	}

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	err := contextService.Create(context.Background(), config.Context{
		Name:           "dev",
		Repository:     validFilesystemRepository(),
		ResourceServer: validResourceServer(),
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
		Name:           "dev",
		ResourceServer: validResourceServer(),
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{BaseDir: "/tmp/dev"},
		},
	}
	if err := contextService.Create(context.Background(), dev); err != nil {
		t.Fatalf("Create(dev) returned error: %v", err)
	}

	prod := config.Context{
		Name:           "prod",
		ResourceServer: validResourceServer(),
		Repository: config.Repository{
			ResourceFormat: config.ResourceFormatYAML,
			Filesystem:     &config.FilesystemRepository{BaseDir: "/tmp/prod"},
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
		Name:           "stage",
		ResourceServer: validResourceServer(),
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
		t.Fatalf("expected updated filesystem base-dir, got %#v", resolved.Repository.Filesystem)
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
			Name:           name,
			ResourceServer: validResourceServer(),
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

func TestResourceFormatDefaultsToJSONOnCreate(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	if err := contextService.Create(context.Background(), config.Context{
		Name:           "dev",
		ResourceServer: validResourceServer(),
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
	if contextCatalog.Contexts[0].Repository.ResourceFormat != config.ResourceFormatJSON {
		t.Fatalf("expected default resource-format json, got %q", contextCatalog.Contexts[0].Repository.ResourceFormat)
	}
}

func TestMetadataBaseDirMatchingRepositoryIsNotPersisted(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	contextService := NewFileContextService(path)

	if err := contextService.Create(context.Background(), config.Context{
		Name:           "dev",
		ResourceServer: validResourceServer(),
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
		t.Fatalf("expected metadata block to be omitted when base-dir matches repository base-dir, got:\n%s", string(raw))
	}

	contextCatalog, err := decodeCatalogFile(path)
	if err != nil {
		t.Fatalf("decodeCatalogFile returned error: %v", err)
	}
	if len(contextCatalog.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(contextCatalog.Contexts))
	}
	if contextCatalog.Contexts[0].Metadata.BaseDir != "" {
		t.Fatalf("expected persisted metadata base-dir to be empty, got %q", contextCatalog.Contexts[0].Metadata.BaseDir)
	}
}

func TestResolveContextDefaultsResourceFormatWhenMissing(t *testing.T) {
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
	if resolved.Repository.ResourceFormat != config.ResourceFormatJSON {
		t.Fatalf("expected resolved default resource-format json, got %q", resolved.Repository.ResourceFormat)
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
		t.Fatalf("expected metadata base-dir default /tmp/repo, got %q", resolved.Metadata.BaseDir)
	}
}

func TestResolveContextOverrideSupportsResourceServerWhenConfigured(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contexts.yaml")
	if err := os.WriteFile(path, []byte(providerSelectionContextCatalogYAML), 0o600); err != nil {
		t.Fatalf("failed to write test contextCatalog: %v", err)
	}

	contextService := NewFileContextService(path)
	resolved, err := contextService.ResolveContext(context.Background(), config.ContextSelection{
		Name:      "fs",
		Overrides: map[string]string{"resource-server.http.base-url": "https://override.example.com"},
	})
	if err != nil {
		t.Fatalf("expected resource-server override to succeed, got %v", err)
	}
	if resolved.ResourceServer == nil || resolved.ResourceServer.HTTP == nil {
		t.Fatalf("expected resource-server configuration, got %#v", resolved.ResourceServer)
	}
	if resolved.ResourceServer.HTTP.BaseURL != "https://override.example.com" {
		t.Fatalf("expected resource-server base-url override, got %q", resolved.ResourceServer.HTTP.BaseURL)
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
			"repository.git.local.base-dir": "/tmp/git",
			"aaa.unknown":                   "x",
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
					Name:           "missing",
					ResourceServer: validResourceServer(),
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
		ResourceFormat: config.ResourceFormatJSON,
		Filesystem:     &config.FilesystemRepository{BaseDir: "/tmp/repo"},
	}
}

func validResourceServer() *config.ResourceServer {
	return &config.ResourceServer{
		HTTP: &config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "secret-token"},
			},
		},
	}
}

const validContextCatalogYAML = `
contexts:
  - name: dev
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    resource-server:
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

const providerSelectionContextCatalogYAML = `
contexts:
  - name: fs
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    resource-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token

  - name: git
    repository:
      resource-format: json
      git:
        local:
          base-dir: /tmp/repo
    resource-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token

  - name: http
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    resource-server:
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
    resource-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token
    secret-store:
      file:
        path: /tmp/secrets.json
        passphrase: change-me

  - name: vault-secret
    repository:
      resource-format: json
      filesystem:
        base-dir: /tmp/repo
    resource-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token
    secret-store:
      vault:
        address: https://vault.example.com
        auth:
          token: s.xxxx

current-ctx: fs
`

const contextCatalogWithoutResourceFormatYAML = `
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/repo
    resource-server:
      http:
        base-url: https://example.com/api
        auth:
          bearer-token:
            token: secret-token
current-ctx: dev
`
