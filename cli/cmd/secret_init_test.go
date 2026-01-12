package cmd_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSecretInitCreatesFileWhenRepositoryInitFails(t *testing.T) {
	home := setTempHome(t)
	contextPath := filepath.Join(home, "context.yaml")
	repoDir := filepath.Join(home, "repo-file")
	secretsPath := filepath.Join(home, "secrets.json")

	if err := os.WriteFile(repoDir, []byte("repo placeholder"), 0o644); err != nil {
		t.Fatalf("write repo placeholder: %v", err)
	}

	writeContextConfigWithSecrets(t, contextPath, repoDir, secretsPath, "test-passphrase")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "secret", "init")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	info, err := os.Stat(secretsPath)
	if err != nil {
		t.Fatalf("stat secrets file: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected secrets file, got directory")
	}
	if info.Size() == 0 {
		t.Fatalf("expected secrets file to contain data")
	}
}
