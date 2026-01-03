package cmd_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cli "declarest/cli/cmd"

	"gopkg.in/yaml.v3"
)

func TestResourceGetRequiresPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "get")
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

func TestResourceListRejectsRepoAndRemote(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "resource", "list")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("repo", "true"); err != nil {
		t.Fatalf("set repo: %v", err)
	}
	if err := command.Flags().Set("remote", "true"); err != nil {
		t.Fatalf("set remote: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestMetadataGetRequiresPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "metadata", "get")
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

func TestMetadataUpdateResourcesRequiresPath(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "metadata", "update-resources")
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

func TestSecretAddRequiresValue(t *testing.T) {
	root := newRootCommand()
	command := findCommand(t, root, "secret", "add")
	var errBuf bytes.Buffer
	command.SetOut(io.Discard)
	command.SetErr(&errBuf)

	if err := command.Flags().Set("path", "/items/foo"); err != nil {
		t.Fatalf("set path: %v", err)
	}
	if err := command.Flags().Set("key", "secret"); err != nil {
		t.Fatalf("set key: %v", err)
	}

	err := command.RunE(command, []string{})
	if err == nil || !cli.IsHandledError(err) {
		t.Fatalf("expected handled error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", errBuf.String())
	}
}

func TestConfigAddContextCreatesStoreAndSetsCurrent(t *testing.T) {
	home := setTempHome(t)

	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")

	root := newRootCommand()
	addCmd := findCommand(t, root, "config", "add")
	addCmd.SetOut(io.Discard)
	addCmd.SetErr(io.Discard)

	if err := addCmd.RunE(addCmd, []string{"test", contextPath}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	storePath := filepath.Join(home, ".declarest", "config")
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read config store: %v", err)
	}

	var store struct {
		Contexts []struct {
			Name string `yaml:"name"`
		} `yaml:"contexts"`
		CurrentContext string `yaml:"currentContext"`
	}
	if err := yaml.Unmarshal(data, &store); err != nil {
		t.Fatalf("unmarshal config store: %v", err)
	}
	if store.CurrentContext != "test" {
		t.Fatalf("expected currentContext to be test, got %q", store.CurrentContext)
	}
	if len(store.Contexts) != 1 || store.Contexts[0].Name != "test" {
		t.Fatalf("expected one context named test, got %#v", store.Contexts)
	}

	listCmd := findCommand(t, root, "config", "list")
	var outBuf bytes.Buffer
	listCmd.SetOut(&outBuf)
	listCmd.SetErr(io.Discard)
	if err := listCmd.RunE(listCmd, []string{}); err != nil {
		t.Fatalf("list RunE: %v", err)
	}
	if !strings.Contains(outBuf.String(), "* test (current)") {
		t.Fatalf("expected current context marker, got %q", outBuf.String())
	}
}

func TestRepoInitUsesFilesystemContext(t *testing.T) {
	home := setTempHome(t)

	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "repo", "init")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	if err := command.RunE(command, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if _, err := os.Stat(repoDir); err != nil {
		t.Fatalf("expected repo directory to exist: %v", err)
	}
}

func TestHelpFlagIsGlobalForRoot(t *testing.T) {
	root := newRootCommand()
	usage := root.UsageString()
	globalIndex := strings.Index(usage, "Global Flags:")
	if globalIndex == -1 {
		t.Fatalf("expected Global Flags section, got %q", usage)
	}
	if !strings.Contains(usage[globalIndex:], "--help") {
		t.Fatalf("expected help flag in Global Flags, got %q", usage)
	}
	if strings.Contains(usage, "\nFlags:\n") {
		localIndex := strings.Index(usage, "\nFlags:\n")
		if localIndex != -1 && strings.Contains(usage[localIndex:globalIndex], "--help") {
			t.Fatalf("expected help flag to be global, got %q", usage)
		}
	}
}

func TestHelpFlagIsGlobalForSubcommand(t *testing.T) {
	root := newRootCommand()
	command, _, err := root.Find([]string{"resource", "get"})
	if err != nil {
		t.Fatalf("find resource get: %v", err)
	}
	usage := command.UsageString()
	globalIndex := strings.Index(usage, "Global Flags:")
	if globalIndex == -1 {
		t.Fatalf("expected Global Flags section, got %q", usage)
	}
	if !strings.Contains(usage[globalIndex:], "--help") {
		t.Fatalf("expected help flag in Global Flags, got %q", usage)
	}
	localIndex := strings.Index(usage, "\nFlags:\n")
	if localIndex == -1 {
		t.Fatalf("expected Flags section for resource get, got %q", usage)
	}
	if localIndex < globalIndex && strings.Contains(usage[localIndex:globalIndex], "--help") {
		t.Fatalf("expected help flag to be global, got %q", usage)
	}
}
