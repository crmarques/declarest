package cmd_test

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	cli "declarest/cli/cmd"
	"declarest/internal/context"
)

func TestConfigDeletePromptsWithoutYes(t *testing.T) {
	home := setTempHome(t)

	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	cmd := findCommand(t, root, "config", "delete")
	cmd.SetOut(io.Discard)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader("n\n"))

	err := cmd.RunE(cmd, []string{"test"})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}

	manager := &context.DefaultContextManager{}
	names, err := manager.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(names) != 1 || names[0] != "test" {
		t.Fatalf("expected context to remain, got %v", names)
	}
}

func TestSecretDeletePromptsWithoutYes(t *testing.T) {
	root := newRootCommand()
	cmd := findCommand(t, root, "secret", "delete")
	cmd.SetOut(io.Discard)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader("n\n"))

	err := cmd.RunE(cmd, []string{"/items/foo", "secret"})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
}

func TestRepoResetPromptsWithoutYes(t *testing.T) {
	root := newRootCommand()
	cmd := findCommand(t, root, "repo", "reset")
	cmd.SetOut(io.Discard)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader("n\n"))

	err := cmd.RunE(cmd, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
}

func TestRepoForcePushPromptsWithoutYes(t *testing.T) {
	root := newRootCommand()
	cmd := findCommand(t, root, "repo", "push")
	cmd.SetOut(io.Discard)
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader("n\n"))

	if err := cmd.Flags().Set("force", "true"); err != nil {
		t.Fatalf("set force: %v", err)
	}

	err := cmd.RunE(cmd, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
}
