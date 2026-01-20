package cmd_test

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	cli "github.com/crmarques/declarest/cli/cmd"
	"github.com/crmarques/declarest/resource"
)

func TestDeleteDefaultsRepoOnly(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "delete")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	repo, err := command.Flags().GetBool("repo")
	if err != nil {
		t.Fatalf("get repo: %v", err)
	}
	remote, err := command.Flags().GetBool("remote")
	if err != nil {
		t.Fatalf("get remote: %v", err)
	}

	if !repo {
		t.Fatalf("expected repo to default to true")
	}
	if remote {
		t.Fatalf("expected remote to default to false")
	}
}

func TestResourceDeleteRemotePromptsWithoutYes(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "delete")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)
	command.SetIn(strings.NewReader("n\n"))

	if err := command.Flags().Set("repo", "false"); err != nil {
		t.Fatalf("set repo: %v", err)
	}
	if err := command.Flags().Set("remote", "true"); err != nil {
		t.Fatalf("set remote: %v", err)
	}

	err := command.RunE(command, []string{"/items/foo"})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(err.Error(), "operation cancelled") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("unexpected usage output, got %q", errBuf.String())
	}
}

func TestResourceDeleteRejectsNoTargets(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "delete")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("remote", "false"); err != nil {
		t.Fatalf("set remote: %v", err)
	}
	if err := command.Flags().Set("repo", "false"); err != nil {
		t.Fatalf("set repo: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestResourceDeleteRepoOnlySkipsRemote(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "resource", "delete")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("repo", "true"); err != nil {
		t.Fatalf("set repo: %v", err)
	}
	if err := command.Flags().Set("remote", "false"); err != nil {
		t.Fatalf("set remote: %v", err)
	}
	if err := command.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set yes: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
}

func TestResourceDeleteRemoteRequiresServer(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "resource", "delete")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("repo", "false"); err != nil {
		t.Fatalf("set repo: %v", err)
	}
	if err := command.Flags().Set("remote", "true"); err != nil {
		t.Fatalf("set remote: %v", err)
	}
	if err := command.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set yes: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if cli.IsHandledError(err) {
		t.Fatalf("expected non-usage error, got %v", err)
	}
}

func TestResourceCreateRejectsAllWithPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "create")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set all: %v", err)
	}
	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestResourceUpdateRequiresPathOrAll(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "update")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestResourceSaveAsOneResourceRequiresCollectionPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "save")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("as-one-resource", "true"); err != nil {
		t.Fatalf("set as-one-resource: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(err.Error(), "--as-one-resource requires a collection path") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestResourceSaveWithSecretsRequiresForce(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "save")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("with-secrets", "true"); err != nil {
		t.Fatalf("set with-secrets: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || cli.IsHandledError(err) {
		t.Fatalf("expected plain error, got %v", err)
	}
	if !strings.Contains(err.Error(), "refusing to save plaintext secrets without --force") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestResourceExplainRequiresPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "explain")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestResourceCreateWithPathSkipsUsageError(t *testing.T) {
	setTempHome(t)

	root := newRootCommand()
	command := findCommand(t, root, "resource", "create")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if cli.IsHandledError(err) {
		t.Fatalf("expected non-usage error, got %v", err)
	}
}

func TestPrintPatchSummaryOutputsOneLinePerOp(t *testing.T) {
	command := newRootCommand()
	var buf bytes.Buffer
	command.SetOut(&buf)

	patch := resource.ResourcePatch{
		{Op: "add", Path: "/items/alpha"},
		{Op: "replace", Path: "/items/beta"},
		{Op: "remove", Path: ""},
	}

	if err := cli.PrintPatchSummary(command, patch); err != nil {
		t.Fatalf("PrintPatchSummary: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != len(patch) {
		t.Fatalf("expected %d lines, got %d", len(patch), len(lines))
	}
	if lines[0] != "add /items/alpha" {
		t.Fatalf("unexpected first line: %q", lines[0])
	}
	if lines[1] != "replace /items/beta" {
		t.Fatalf("unexpected second line: %q", lines[1])
	}
	if lines[2] != "remove" {
		t.Fatalf("unexpected third line: %q", lines[2])
	}
}
