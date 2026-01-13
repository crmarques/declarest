package cmd_test

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	ctx "declarest/internal/context"
)

func TestConfigEnvCommandDefaults(t *testing.T) {
	home := setTempHome(t)
	t.Setenv(ctx.ConfigDirEnvVar, "")
	t.Setenv(ctx.ConfigFileEnvVar, "")

	var out bytes.Buffer
	cmd := findCommand(t, newRootCommand(), "config", "env")
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	output := out.String()
	dirInfo, err := ctx.ConfigDirPathInfo()
	if err != nil {
		t.Fatalf("ConfigDirPathInfo: %v", err)
	}
	fileInfo, err := ctx.ConfigFilePathInfo()
	if err != nil {
		t.Fatalf("ConfigFilePathInfo: %v", err)
	}

	if !strings.Contains(output, ctx.ConfigDirEnvVar) {
		t.Fatalf("missing config dir name in output: %q", output)
	}
	if !strings.Contains(output, dirInfo.Path) {
		t.Fatalf("missing config dir path %q: %q", dirInfo.Path, output)
	}
	if !strings.Contains(output, "default (DECLAREST_HOME/.declarest)") {
		t.Fatalf("unexpected source for config dir: %q", output)
	}

	if !strings.Contains(output, ctx.ConfigFileEnvVar) {
		t.Fatalf("missing config file name in output: %q", output)
	}
	if !strings.Contains(output, fileInfo.Path) {
		t.Fatalf("missing config file path %q: %q", fileInfo.Path, output)
	}
	if !strings.Contains(output, "default (DECLAREST_HOME/.declarest/config)") {
		t.Fatalf("unexpected source for config file: %q", output)
	}

	expectedDir := filepath.Join(home, ".declarest")
	if dirInfo.Path != expectedDir {
		t.Fatalf("expected default config dir %q, got %q", expectedDir, dirInfo.Path)
	}
	expectedFile := filepath.Join(expectedDir, "config")
	if fileInfo.Path != expectedFile {
		t.Fatalf("expected default config file %q, got %q", expectedFile, fileInfo.Path)
	}
}

func TestConfigEnvCommandConfigDirOverride(t *testing.T) {
	setTempHome(t)
	dir := filepath.Join(t.TempDir(), "custom-dir")
	t.Setenv(ctx.ConfigDirEnvVar, dir)
	t.Setenv(ctx.ConfigFileEnvVar, "")

	var out bytes.Buffer
	cmd := findCommand(t, newRootCommand(), "config", "env")
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, dir) {
		t.Fatalf("expected custom dir %q in output: %q", dir, output)
	}
	if !strings.Contains(output, "environment (DECLAREST_CONFIG_DIR)") {
		t.Fatalf("expected environment source for config dir: %q", output)
	}
	if !strings.Contains(output, "default (derived from DECLAREST_CONFIG_DIR)") {
		t.Fatalf("expected derived source for config file: %q", output)
	}
}

func TestConfigEnvCommandConfigFileOverride(t *testing.T) {
	home := setTempHome(t)
	t.Setenv(ctx.ConfigDirEnvVar, "")
	file := filepath.Join(home, "custom-dir", "declarest.conf")
	t.Setenv(ctx.ConfigFileEnvVar, file)

	var out bytes.Buffer
	cmd := findCommand(t, newRootCommand(), "config", "env")
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, file) {
		t.Fatalf("expected custom file %q in output: %q", file, output)
	}
	if !strings.Contains(output, "environment (DECLAREST_CONFIG_FILE)") {
		t.Fatalf("expected environment source for config file: %q", output)
	}
	if !strings.Contains(output, "default (DECLAREST_HOME/.declarest)") {
		t.Fatalf("expected default source for config dir: %q", output)
	}
}
