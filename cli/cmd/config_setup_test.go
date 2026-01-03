package cmd_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"declarest/internal/context"

	"gopkg.in/yaml.v3"
)

type configSetupStore struct {
	Contexts []struct {
		Name    string                 `yaml:"name"`
		Context *context.ContextConfig `yaml:"context"`
	} `yaml:"contexts"`
	CurrentContext string `yaml:"currentContext"`
}

func readConfigSetupStore(t *testing.T, home string) configSetupStore {
	t.Helper()

	storePath := filepath.Join(home, ".declarest", "config")
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read config store: %v", err)
	}

	var store configSetupStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		t.Fatalf("unmarshal config store: %v", err)
	}
	return store
}

func TestConfigSetupFilesystem(t *testing.T) {
	home := setTempHome(t)

	root := newRootCommand()
	cmd := findCommand(t, root, "config", "init")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	repoDir := filepath.Join(home, "repo")
	input := fmt.Sprintf("test\nfilesystem\n%s\n\n\n", repoDir)
	cmd.SetIn(strings.NewReader(input))

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	store := readConfigSetupStore(t, home)
	if store.CurrentContext != "test" {
		t.Fatalf("expected currentContext to be test, got %q", store.CurrentContext)
	}
	if len(store.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(store.Contexts))
	}
	cfg := store.Contexts[0].Context
	if cfg == nil || cfg.Repository == nil || cfg.Repository.Filesystem == nil {
		t.Fatalf("expected filesystem repository config, got %#v", cfg)
	}
	if cfg.Repository.Filesystem.BaseDir != repoDir {
		t.Fatalf("expected base_dir %q, got %q", repoDir, cfg.Repository.Filesystem.BaseDir)
	}
	if cfg.Repository.Git != nil {
		t.Fatalf("expected git repository config to be nil, got %#v", cfg.Repository.Git)
	}
	if cfg.ManagedServer != nil {
		t.Fatalf("expected managed server config to be nil, got %#v", cfg.ManagedServer)
	}
}

func TestConfigSetupGitRemote(t *testing.T) {
	home := setTempHome(t)

	root := newRootCommand()
	cmd := findCommand(t, root, "config", "init")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	repoDir := filepath.Join(home, "repo")
	remoteURL := "https://example.com/repo.git"
	input := fmt.Sprintf("remote\n%s\n%s\n%s\nnone\n\n\n\n\n", "git-remote", repoDir, remoteURL)
	cmd.SetIn(strings.NewReader(input))

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	store := readConfigSetupStore(t, home)
	if store.CurrentContext != "remote" {
		t.Fatalf("expected currentContext to be remote, got %q", store.CurrentContext)
	}
	if len(store.Contexts) != 1 {
		t.Fatalf("expected one context, got %d", len(store.Contexts))
	}
	cfg := store.Contexts[0].Context
	if cfg == nil || cfg.Repository == nil || cfg.Repository.Git == nil {
		t.Fatalf("expected git repository config, got %#v", cfg)
	}
	if cfg.Repository.Filesystem != nil {
		t.Fatalf("expected filesystem repository config to be nil, got %#v", cfg.Repository.Filesystem)
	}
	if cfg.Repository.Git.Local == nil || cfg.Repository.Git.Local.BaseDir != repoDir {
		t.Fatalf("expected local base_dir %q, got %#v", repoDir, cfg.Repository.Git.Local)
	}
	if cfg.Repository.Git.Remote == nil || cfg.Repository.Git.Remote.URL != remoteURL {
		t.Fatalf("expected remote url %q, got %#v", remoteURL, cfg.Repository.Git.Remote)
	}
	if cfg.ManagedServer != nil {
		t.Fatalf("expected managed server config to be nil, got %#v", cfg.ManagedServer)
	}
}
