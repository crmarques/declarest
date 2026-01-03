package cmd_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cli "declarest/cli/cmd"
)

func TestSecretListDefaultOutput(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/alpha", "alpha", "a-secret")
	addSecret(t, "/projects/beta", "token", "token-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "list")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("/projects/alpha:\n  alpha\n  zeta\n/projects/beta:\n  token")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretListResourcePathsOnly(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/beta", "token", "token-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "list")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("paths-only", "true"); err != nil {
		t.Fatalf("set paths-only: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("/projects/alpha\n/projects/beta")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretListShowSecrets(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/alpha", "alpha", "a-secret")
	addSecret(t, "/projects/beta", "token", "token-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "list")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("show-secrets", "true"); err != nil {
		t.Fatalf("set show-secrets: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("/projects/alpha:\n  alpha:a-secret\n  zeta:z-secret\n/projects/beta:\n  token:token-secret")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretListShowSecretsForPath(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/alpha", "alpha", "a-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "list")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/projects/alpha"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("show-secrets", "true"); err != nil {
		t.Fatalf("set show-secrets: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("/projects/alpha:\n  alpha:a-secret\n  zeta:z-secret")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretListRejectsConflictingFlags(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "alpha", "a-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "list")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("paths-only", "true"); err != nil {
		t.Fatalf("set paths-only: %v", err)
	}
	if err := command.Flags().Set("show-secrets", "true"); err != nil {
		t.Fatalf("set show-secrets: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func setupSecretContext(t *testing.T) {
	t.Helper()
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	secretsPath := filepath.Join(home, "secrets.json")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfigWithSecrets(t, contextPath, repoDir, secretsPath, "test-passphrase")
	addContext(t, "test", contextPath)
}

func writeContextConfigWithSecrets(t *testing.T, path, repoDir, secretsPath, passphrase string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "repository:\n  filesystem:\n    base_dir: %s\n", repoDir)
	fmt.Fprintf(&b, "secret_manager:\n  file:\n    path: %s\n    passphrase: %s\n", secretsPath, passphrase)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write context config: %v", err)
	}
}

func addSecret(t *testing.T, resourcePath, key, value string) {
	t.Helper()
	root := newRootCommand()
	command := findCommand(t, root, "secret", "add")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", resourcePath); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("key", key); err != nil {
		t.Fatalf("set key: %v", err)
	}
	if err := command.Flags().Set("value", value); err != nil {
		t.Fatalf("set value: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
}
