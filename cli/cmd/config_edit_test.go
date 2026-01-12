package cmd_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestConfigEditRejectsConflictingRepositories(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo-conflict")
	contextPath := filepath.Join(home, "context-conflict.yaml")
	writeContextConfig(t, contextPath, repoDir, "https://example.com/api")
	addContext(t, "conflict", contextPath)

	editorPath := filepath.Join(home, "invalid-edit.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

cat > "$1" <<'EOF'
repository:
  filesystem:
    base_dir: %s
  git:
    local:
      base_dir: %s
managed_server:
  http:
    base_url: https://example.com/api
EOF
`, repoDir, repoDir)
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

	err := command.RunE(command, []string{"conflict"})
	if err == nil {
		t.Fatalf("expected edit command to reject invalid config")
	}
	if !strings.Contains(err.Error(), "repository configuration must define either git or filesystem, not both") {
		t.Fatalf("unexpected error: %v", err)
	}

	store := readConfigSetupStore(t, home)
	cfg := findContextConfig(t, store, "conflict")
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.BaseURL != "https://example.com/api" {
		t.Fatalf("context should remain unchanged after validation failure")
	}
}

func TestConfigEditPrepopulatesExistingAttributes(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo4")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "https://example.com/api")
	addContext(t, "preload", contextPath)

	captured := filepath.Join(home, "captured.yaml")
	editorPath := filepath.Join(home, "capture.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cp "$1" %q
`, captured)
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

	if err := command.RunE(command, []string{"preload"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("read captured config: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "base_dir: "+repoDir) {
		t.Fatalf("expected filesystem base_dir in editor payload, got:\n%s", output)
	}
	if !strings.Contains(output, "base_url: https://example.com/api") {
		t.Fatalf("expected managed server base_url, got:\n%s", output)
	}
}

func TestConfigEditLoadsFromConfigFile(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo5")
	contextPath := filepath.Join(home, "context-file.yaml")
	writeContextConfig(t, contextPath, repoDir, "https://example.com/api")

	captured := filepath.Join(home, "captured-file.yaml")
	editorPath := filepath.Join(home, "capture-file.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cp "$1" %q
`, captured)
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

	if err := command.RunE(command, []string{contextPath}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("read captured config: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, "base_dir: "+repoDir) {
		t.Fatalf("expected filesystem base_dir in editor payload, got:\n%s", output)
	}
	if !strings.Contains(output, "base_url: https://example.com/api") {
		t.Fatalf("expected managed server base_url, got:\n%s", output)
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
