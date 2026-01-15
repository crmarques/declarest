package cmd_test

import (
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

func TestConfigAddInteractiveFilesystem(t *testing.T) {
	home := setTempHome(t)

	root := newRootCommand()
	cmd := findCommand(t, root, "config", "add")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	repoDir := filepath.Join(home, "repo")
	baseURL := "http://localhost:1234"
	inputParts := []string{
		"test",
		"",
		repoDir,
		baseURL,
		"",
		"",
		"",
		"",
	}
	input := strings.Join(inputParts, "\n") + "\n"
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
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.BaseURL != baseURL {
		t.Fatalf("expected managed server base URL %q, got %q", baseURL, cfg.ManagedServer.HTTP.BaseURL)
	}
	if cfg.ManagedServer.HTTP.Auth != nil {
		t.Fatalf("expected no managed server auth, got %#v", cfg.ManagedServer.HTTP.Auth)
	}
}

func TestConfigAddInteractiveGitRemote(t *testing.T) {
	home := setTempHome(t)

	root := newRootCommand()
	cmd := findCommand(t, root, "config", "add")
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	repoDir := filepath.Join(home, "repo")
	remoteURL := "https://example.com/repo.git"
	baseURL := "http://localhost:1234"
	inputParts := []string{
		"remote",
		"git-remote",
		repoDir,
		remoteURL,
		"",
		"",
		"",
		"",
		baseURL,
		"",
		"",
		"",
		"",
	}
	input := strings.Join(inputParts, "\n") + "\n"
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
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.BaseURL != baseURL {
		t.Fatalf("expected managed server base URL %q, got %q", baseURL, cfg.ManagedServer.HTTP.BaseURL)
	}
	if cfg.ManagedServer.HTTP.Auth != nil {
		t.Fatalf("expected no managed server auth, got %#v", cfg.ManagedServer.HTTP.Auth)
	}
}
