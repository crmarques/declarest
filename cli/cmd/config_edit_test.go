package cmd_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"declarest/internal/context"
)

func TestConfigEditUpdatesExisting(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "edit-test", contextPath)

	editorPath := filepath.Join(home, "edit.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail

if ! grep -q "resource_format: json" "$1"; then
  echo "missing default resource_format" >&2
  exit 1
fi

if ! grep -q "auto_sync: true" "$1"; then
  echo "missing default auto_sync" >&2
  exit 1
fi

sed -i 's|base_url: \"\"|base_url: https://example.com/api|' "$1"
`
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "config", "edit")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("editor", editorPath); err != nil {
		t.Fatalf("set editor: %v", err)
	}

	if err := command.RunE(command, []string{"edit-test"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	store := readConfigSetupStore(t, home)
	cfg := findContextConfig(t, store, "edit-test")
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.BaseURL != "https://example.com/api" {
		t.Fatalf("expected base_url to be updated, got %q", cfg.ManagedServer.HTTP.BaseURL)
	}
	if cfg.ManagedServer.HTTP.Auth != nil {
		t.Fatalf("expected managed server auth to be nil, got %#v", cfg.ManagedServer.HTTP.Auth)
	}
	if cfg.Repository == nil {
		t.Fatalf("expected repository config")
	}
	if cfg.Repository.ResourceFormat != "" {
		t.Fatalf("expected default resource_format to be stripped, got %q", cfg.Repository.ResourceFormat)
	}
}

func TestConfigEditAddsContext(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo2")

	editorPath := filepath.Join(home, "edit-new.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

cat > "$1" <<'EOF'
repository:
  filesystem:
    base_dir: %s
managed_server:
  http:
    base_url: https://example.com/api
EOF
`, repoDir)
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "config", "edit")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("editor", editorPath); err != nil {
		t.Fatalf("set editor: %v", err)
	}

	if err := command.RunE(command, []string{"new-context"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	store := readConfigSetupStore(t, home)
	cfg := findContextConfig(t, store, "new-context")
	if cfg.Repository == nil || cfg.Repository.Filesystem == nil {
		t.Fatalf("expected filesystem repository config, got %#v", cfg.Repository)
	}
	if cfg.Repository.Filesystem.BaseDir != repoDir {
		t.Fatalf("expected base_dir %q, got %q", repoDir, cfg.Repository.Filesystem.BaseDir)
	}
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.BaseURL != "https://example.com/api" {
		t.Fatalf("expected base_url to be set, got %q", cfg.ManagedServer.HTTP.BaseURL)
	}
}

func findContextConfig(t *testing.T, store configSetupStore, name string) *context.ContextConfig {
	t.Helper()
	for _, entry := range store.Contexts {
		if entry.Name == name {
			return entry.Context
		}
	}
	t.Fatalf("context %q not found in store", name)
	return nil
}
