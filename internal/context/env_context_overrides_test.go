package context

import (
	"path/filepath"
	"strings"
	"testing"

	"declarest/internal/reconciler"
	"declarest/internal/repository"
)

func newTempManager(t *testing.T) *DefaultContextManager {
	t.Helper()
	return &DefaultContextManager{ConfigFilePath: filepath.Join(t.TempDir(), "config.yaml")}
}

func requireFileSystemRepo(t *testing.T, recon *reconciler.DefaultReconciler) *repository.FileSystemResourceRepositoryManager {
	t.Helper()
	fs, ok := recon.ResourceRepositoryManager.(*repository.FileSystemResourceRepositoryManager)
	if !ok || fs == nil {
		t.Fatalf("expected repository manager to be FileSystemResourceRepositoryManager, got %T", recon.ResourceRepositoryManager)
	}
	return fs
}

func TestLoadContextWithEnvOverridesExistingContext(t *testing.T) {
	manager := newTempManager(t)
	baseDir := filepath.Join(t.TempDir(), "repo")
	cfg := &ContextConfig{
		Repository: &RepositoryConfig{
			Filesystem: &repository.FileSystemResourceRepositoryConfig{BaseDir: baseDir},
		},
	}
	if err := manager.AddContextConfig("env", cfg); err != nil {
		t.Fatalf("AddContextConfig: %v", err)
	}

	t.Setenv("DECLAREST_CTX_NAME", "env")
	overridden := filepath.Join(t.TempDir(), "override")
	t.Setenv("DECLAREST_CTX_REPOSITORY_FILESYSTEM_BASE_DIR", overridden)

	ctx, err := LoadContextWithEnv(manager)
	if err != nil {
		t.Fatalf("LoadContextWithEnv: %v", err)
	}
	if ctx.Name != "env" {
		t.Fatalf("unexpected context name %q", ctx.Name)
	}

	recon, ok := ctx.Reconciler.(*reconciler.DefaultReconciler)
	if !ok {
		t.Fatalf("unexpected reconciler type %T", ctx.Reconciler)
	}

	fs := requireFileSystemRepo(t, recon)
	if fs.BaseDir != overridden {
		t.Fatalf("expected repository base dir %q, got %q", overridden, fs.BaseDir)
	}
}

func TestLoadContextWithEnvBuildsNewContextFromOverrides(t *testing.T) {
	manager := newTempManager(t)
	envName := "env-only"
	repoDir := filepath.Join(t.TempDir(), "repo")

	t.Setenv("DECLAREST_CTX_NAME", envName)
	t.Setenv("DECLAREST_CTX_REPOSITORY_FILESYSTEM_BASE_DIR", repoDir)

	ctx, err := LoadContextWithEnv(manager)
	if err != nil {
		t.Fatalf("LoadContextWithEnv: %v", err)
	}
	if ctx.Name != envName {
		t.Fatalf("unexpected context name %q", ctx.Name)
	}

	recon, ok := ctx.Reconciler.(*reconciler.DefaultReconciler)
	if !ok {
		t.Fatalf("unexpected reconciler type %T", ctx.Reconciler)
	}

	fs := requireFileSystemRepo(t, recon)
	if fs.BaseDir != repoDir {
		t.Fatalf("expected repository base dir %q, got %q", repoDir, fs.BaseDir)
	}
}

func TestLoadContextWithEnvMissingOverrides(t *testing.T) {
	manager := newTempManager(t)
	t.Setenv("DECLAREST_CTX_NAME", "missing")

	if _, err := LoadContextWithEnv(manager); err == nil {
		t.Fatal("expected error when overrides are missing")
	}
}

func TestLoadContextWithEnvUnsupportedOverride(t *testing.T) {
	manager := newTempManager(t)
	t.Setenv("DECLAREST_CTX_BOGUS", "value")

	_, err := LoadContextWithEnv(manager)
	if err == nil {
		t.Fatal("expected error for unsupported override")
	}
	if !strings.Contains(err.Error(), "unsupported context override") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadContextWithEnvResolvesPlaceholders(t *testing.T) {
	manager := newTempManager(t)
	envName := "env-placeholder-success"
	repoDir := filepath.Join(t.TempDir(), "repo")
	cfg := &ContextConfig{
		Repository: &RepositoryConfig{
			Filesystem: &repository.FileSystemResourceRepositoryConfig{BaseDir: "${DECLAREST_ENV_PLACEHOLDER_REPO_DIR}"},
		},
	}
	if err := manager.AddContextConfig(envName, cfg); err != nil {
		t.Fatalf("AddContextConfig: %v", err)
	}

	t.Setenv("DECLAREST_CTX_NAME", envName)
	t.Setenv("DECLAREST_ENV_PLACEHOLDER_REPO_DIR", repoDir)

	ctx, err := LoadContextWithEnv(manager)
	if err != nil {
		t.Fatalf("LoadContextWithEnv: %v", err)
	}
	recon, ok := ctx.Reconciler.(*reconciler.DefaultReconciler)
	if !ok {
		t.Fatalf("unexpected reconciler type %T", ctx.Reconciler)
	}

	fs := requireFileSystemRepo(t, recon)
	if fs.BaseDir != repoDir {
		t.Fatalf("expected repository base dir %q, got %q", repoDir, fs.BaseDir)
	}
}

func TestLoadContextWithEnvMissingPlaceholderVariable(t *testing.T) {
	manager := newTempManager(t)
	envName := "env-placeholder-missing"
	cfg := &ContextConfig{
		Repository: &RepositoryConfig{
			Filesystem: &repository.FileSystemResourceRepositoryConfig{BaseDir: "${DECLAREST_ENV_PLACEHOLDER_REPO_DIR}"},
		},
	}
	if err := manager.AddContextConfig(envName, cfg); err != nil {
		t.Fatalf("AddContextConfig: %v", err)
	}

	t.Setenv("DECLAREST_CTX_NAME", envName)

	if _, err := LoadContextWithEnv(manager); err == nil {
		t.Fatal("expected error when placeholder environment variable is missing")
	} else if !strings.Contains(err.Error(), "DECLAREST_ENV_PLACEHOLDER_REPO_DIR") {
		t.Fatalf("unexpected error: %v", err)
	}
}
