package cmd_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/crmarques/declarest/context"

	"gopkg.in/yaml.v3"
)

func TestConfigPrintTemplateIncludesSections(t *testing.T) {
	root := newRootCommand()
	cmd := findCommand(t, root, "config", "print-template")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var cfg context.ContextConfig
	if err := yaml.Unmarshal(out.Bytes(), &cfg); err != nil {
		t.Fatalf("unmarshal template: %v", err)
	}
	if cfg.Repository == nil || cfg.Repository.Git == nil || cfg.Repository.Git.Local == nil {
		t.Fatalf("expected git repository config, got %#v", cfg.Repository)
	}
	if cfg.ManagedServer == nil || cfg.ManagedServer.HTTP == nil {
		t.Fatalf("expected managed server config, got %#v", cfg.ManagedServer)
	}
	if cfg.ManagedServer.HTTP.Auth == nil || cfg.ManagedServer.HTTP.Auth.OAuth2 == nil {
		t.Fatalf("expected oauth2 auth config, got %#v", cfg.ManagedServer.HTTP.Auth)
	}
	if cfg.SecretManager == nil || cfg.SecretManager.File == nil {
		t.Fatalf("expected secret store config, got %#v", cfg.SecretManager)
	}
	if cfg.Metadata == nil || cfg.Metadata.BaseDir == "" {
		t.Fatalf("expected metadata config, got %#v", cfg.Metadata)
	}
}
