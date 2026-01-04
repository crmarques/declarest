package cmd_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cli "declarest/cli/cmd"
)

func TestSecretCheckFindsUnmappedSecrets(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	writeResourceFile(t, repoDir, "/items/foo", `{"token":"abc","password":"xyz"}`)

	root := newRootCommand()
	command := findCommand(t, root, "secret", "check")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "/items/foo:") {
		t.Fatalf("expected resource path in output, got %q", got)
	}
	if !strings.Contains(got, "password") || !strings.Contains(got, "token") {
		t.Fatalf("expected secret attributes in output, got %q", got)
	}
}

func TestSecretCheckFixRequiresSecretStore(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	writeResourceFile(t, repoDir, "/items/foo", `{"token":"abc"}`)

	root := newRootCommand()
	command := findCommand(t, root, "secret", "check")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("fix", "true"); err != nil {
		t.Fatalf("set fix: %v", err)
	}

	err := command.RunE(command, []string{"/items/foo"})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Secret store is not configured") {
		t.Fatalf("expected configuration guidance, got %q", errBuf.String())
	}
}

func TestSecretCheckSkipsNonSecretTokenFlags(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	writeResourceFile(t, repoDir, "/items/foo", `{
  "attributes": {
    "client.use.lightweight.access.token.enabled": "false",
    "client.secret.creation.time": "1713655874"
  },
  "protocolMappers": [
    {
      "config": {
        "access.token.claim": "true",
        "id.token.claim": "true"
      }
    }
  ],
  "secret": "s3cr3t"
}`)

	root := newRootCommand()
	command := findCommand(t, root, "secret", "check")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "  secret") {
		t.Fatalf("expected secret field, got %q", output)
	}
	if strings.Contains(output, "access.token.claim") {
		t.Fatalf("unexpected access.token.claim warning, got %q", output)
	}
	if strings.Contains(output, "id.token.claim") {
		t.Fatalf("unexpected id.token.claim warning, got %q", output)
	}
	if strings.Contains(output, "client.use.lightweight.access.token.enabled") {
		t.Fatalf("unexpected token enabled warning, got %q", output)
	}
	if strings.Contains(output, "client.secret.creation.time") {
		t.Fatalf("unexpected secret creation time warning, got %q", output)
	}
}

func writeResourceFile(t *testing.T, repoDir, logicalPath, payload string) {
	t.Helper()
	trimmed := strings.TrimPrefix(logicalPath, "/")
	dir := filepath.Join(repoDir, trimmed)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir resource dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "resource.json"), []byte(payload), 0o644); err != nil {
		t.Fatalf("write resource: %v", err)
	}
}
