package cmd_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	cli "declarest/cli/cmd"
	ctx "declarest/internal/context"

	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	return cli.NewRootCommand()
}

func findCommand(t *testing.T, root *cobra.Command, args ...string) *cobra.Command {
	t.Helper()
	found, _, err := root.Find(args)
	if err != nil {
		t.Fatalf("find %v: %v", args, err)
	}
	return found
}

func setTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func writeContextConfig(t *testing.T, path, repoDir, baseURL string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "repository:\n  filesystem:\n    base_dir: %s\n", repoDir)
	if baseURL != "" {
		fmt.Fprintf(&b, "managed_server:\n  http:\n    base_url: %s\n", baseURL)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write context config: %v", err)
	}
}

func addContext(t *testing.T, name, contextPath string) {
	t.Helper()
	manager := &ctx.DefaultContextManager{}
	if err := manager.AddContext(name, contextPath); err != nil {
		t.Fatalf("AddContext: %v", err)
	}
}
