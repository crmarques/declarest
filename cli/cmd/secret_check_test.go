package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"

	cli "github.com/crmarques/declarest/cli/cmd"
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

func TestSecretCheckFixRespectsMetadataWildcardPath(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	specPath := filepath.Join(home, "openapi.yml")
	secretStorePath := filepath.Join(home, "secrets.json")
	passphrase := "change-me"

	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	config := fmt.Sprintf(
		`repository:
  filesystem:
    base_dir: %q
managed_server:
  http:
    base_url: http://example.com
    openapi: %q
secret_store:
  file:
    path: %q
    passphrase: %q
`, repoDir, specPath, secretStorePath, passphrase)

	if err := os.WriteFile(contextPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	addContext(t, "secret-check-wildcard", contextPath)

	root := newRootCommand()
	initCmd := findCommand(t, root, "secret", "init")
	initCmd.SetOut(io.Discard)
	initCmd.SetErr(io.Discard)
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("secret init: %v", err)
	}

	writeResourceFile(t, repoDir, "/fruits/apple", `{"password":"s3cr3t"}`)

	checkCmd := findCommand(t, root, "secret", "check")
	checkCmd.SetOut(io.Discard)
	checkCmd.SetErr(io.Discard)
	if err := checkCmd.Flags().Set("fix", "true"); err != nil {
		t.Fatalf("set fix: %v", err)
	}
	if err := checkCmd.RunE(checkCmd, []string{"/fruits/apple"}); err != nil {
		t.Fatalf("secret check --fix: %v", err)
	}

	metadataPath := filepath.Join(repoDir, "fruits", "_", "_", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	found := false
	for _, attr := range meta.ResourceInfo.SecretInAttributes {
		if attr == "password" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected password to be marked as secret, got %v", meta.ResourceInfo.SecretInAttributes)
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
