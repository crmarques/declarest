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

func TestSecretExportPathCSV(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/alpha", "alpha", "a-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "export")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("path", "/projects/alpha"); err != nil {
		t.Fatalf("set path: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("path,key,value\n/projects/alpha,alpha,a-secret\n/projects/alpha,zeta,z-secret")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretExportPathCSVPositional(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "zeta", "z-secret")
	addSecret(t, "/projects/alpha", "alpha", "a-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "export")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{"/projects/alpha"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("path,key,value\n/projects/alpha,alpha,a-secret\n/projects/alpha,zeta,z-secret")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretExportAllCSV(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "alpha", "a-secret")
	addSecret(t, "/projects/beta", "token", "token-secret")

	root := newRootCommand()
	command := findCommand(t, root, "secret", "export")
	var buf bytes.Buffer
	command.SetOut(&buf)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set all: %v", err)
	}

	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := strings.TrimSpace("path,key,value\n/projects/alpha,alpha,a-secret\n/projects/beta,token,token-secret")
	if got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestSecretImportConflictsAndForce(t *testing.T) {
	setupSecretContext(t)
	addSecret(t, "/projects/alpha", "alpha", "original")

	content := "path,key,value\n/projects/alpha,alpha,new-value\n"
	path := filepath.Join(t.TempDir(), "secrets.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "secret", "import")
	command.SetOut(io.Discard)
	var errBuf bytes.Buffer
	command.SetErr(&errBuf)

	if err := command.Flags().Set("file", path); err != nil {
		t.Fatalf("set file: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Import would overwrite existing secrets") {
		t.Fatalf("expected conflict message, got %q", errBuf.String())
	}

	getCmd := findCommand(t, newRootCommand(), "secret", "get")
	var getOut bytes.Buffer
	getCmd.SetOut(&getOut)
	getCmd.SetErr(io.Discard)
	if err := getCmd.Flags().Set("path", "/projects/alpha"); err != nil {
		t.Fatalf("set get path: %v", err)
	}
	if err := getCmd.Flags().Set("key", "alpha"); err != nil {
		t.Fatalf("set get key: %v", err)
	}
	if err := getCmd.RunE(getCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if strings.TrimSpace(getOut.String()) != "original" {
		t.Fatalf("secret was overwritten without force")
	}

	forceRoot := newRootCommand()
	forceCmd := findCommand(t, forceRoot, "secret", "import")
	forceCmd.SetOut(io.Discard)
	forceCmd.SetErr(io.Discard)
	if err := forceCmd.Flags().Set("file", path); err != nil {
		t.Fatalf("set file: %v", err)
	}
	if err := forceCmd.Flags().Set("force", "true"); err != nil {
		t.Fatalf("set force: %v", err)
	}
	if err := forceCmd.RunE(forceCmd, []string{}); err != nil {
		t.Fatalf("RunE with --force: %v", err)
	}

	verifyCmd := findCommand(t, newRootCommand(), "secret", "get")
	var verifyOut bytes.Buffer
	verifyCmd.SetOut(&verifyOut)
	verifyCmd.SetErr(io.Discard)
	if err := verifyCmd.Flags().Set("path", "/projects/alpha"); err != nil {
		t.Fatalf("set verify path: %v", err)
	}
	if err := verifyCmd.Flags().Set("key", "alpha"); err != nil {
		t.Fatalf("set verify key: %v", err)
	}
	if err := verifyCmd.RunE(verifyCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if strings.TrimSpace(verifyOut.String()) != "new-value" {
		t.Fatalf("force import did not apply new value")
	}
}

func TestSecretImportPositionalFile(t *testing.T) {
	setupSecretContext(t)

	content := "path,key,value\n/projects/alpha,alpha,new-value\n"
	path := filepath.Join(t.TempDir(), "secrets.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "secret", "import")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.RunE(command, []string{path}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	getCmd := findCommand(t, newRootCommand(), "secret", "get")
	var getOut bytes.Buffer
	getCmd.SetOut(&getOut)
	getCmd.SetErr(io.Discard)
	if err := getCmd.Flags().Set("path", "/projects/alpha"); err != nil {
		t.Fatalf("set get path: %v", err)
	}
	if err := getCmd.Flags().Set("key", "alpha"); err != nil {
		t.Fatalf("set get key: %v", err)
	}
	if err := getCmd.RunE(getCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if strings.TrimSpace(getOut.String()) != "new-value" {
		t.Fatalf("expected imported value, got %q", strings.TrimSpace(getOut.String()))
	}
}
