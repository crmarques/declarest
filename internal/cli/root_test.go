package cli

import (
	"bytes"
	"io"
	"testing"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func TestRegisteredCommandCount(t *testing.T) {
	t.Parallel()

	paths := registeredPaths(NewRootCommand(common.CommandWiring{}), nil)
	if len(paths) != 53 {
		t.Fatalf("expected 53 registered command paths, got %d", len(paths))
	}
}

func TestAllCommandPathsPrintPlaceholder(t *testing.T) {
	t.Parallel()

	rootOutput, err := executeForTest()
	if err != nil {
		t.Fatalf("root command returned error: %v", err)
	}
	if rootOutput != common.PlaceholderMessage+"\n" {
		t.Fatalf("root output mismatch: got %q", rootOutput)
	}

	for _, path := range registeredPaths(NewRootCommand(common.CommandWiring{}), nil) {
		path := path
		t.Run(joinPath(path), func(t *testing.T) {
			t.Parallel()

			output, runErr := executeForTest(path...)
			if runErr != nil {
				t.Fatalf("command %q returned error: %v", joinPath(path), runErr)
			}
			if output != common.PlaceholderMessage+"\n" {
				t.Fatalf("command %q output mismatch: got %q", joinPath(path), output)
			}
		})
	}
}

func TestGlobalFlagsParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root_with_all_global_flags",
			args: []string{"--context", "prod", "--debug", "--no-status", "--output", "json"},
		},
		{
			name: "resource_get_with_context_and_output",
			args: []string{"--context", "prod", "--output", "json", "resource", "get"},
		},
		{
			name: "repo_check_with_debug",
			args: []string{"--debug", "repo", "check"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeForTest(tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output != common.PlaceholderMessage+"\n" {
				t.Fatalf("output mismatch: got %q", output)
			}
		})
	}
}

func TestCommandSpecificFlagsParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "resource_delete_flags",
			args: []string{"resource", "delete", "--force", "--recursive"},
		},
		{
			name: "metadata_infer_flags",
			args: []string{"metadata", "infer", "--apply", "--recursive"},
		},
		{
			name: "repo_push_force_flag",
			args: []string{"repo", "push", "--force"},
		},
		{
			name: "repo_reset_hard_flag",
			args: []string{"repo", "reset", "--hard"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeForTest(tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output != common.PlaceholderMessage+"\n" {
				t.Fatalf("output mismatch: got %q", output)
			}
		})
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	t.Parallel()

	_, err := executeForTest("unknown-command")
	if err == nil {
		t.Fatal("expected unknown command to return error")
	}
}

func executeForTest(args ...string) (string, error) {
	command := NewRootCommand(common.CommandWiring{})
	out := &bytes.Buffer{}
	command.SetOut(out)
	command.SetErr(io.Discard)
	command.SetArgs(args)

	err := command.Execute()
	return out.String(), err
}

func registeredPaths(command *cobra.Command, prefix []string) [][]string {
	paths := make([][]string, 0)
	for _, child := range command.Commands() {
		name := child.Name()
		if name == "help" || len(name) > 1 && name[:2] == "__" {
			continue
		}
		current := append(append([]string{}, prefix...), name)
		paths = append(paths, current)
		paths = append(paths, registeredPaths(child, current)...)
	}
	return paths
}

func joinPath(path []string) string {
	if len(path) == 0 {
		return "root"
	}
	joined := path[0]
	for i := 1; i < len(path); i++ {
		joined += " " + path[i]
	}
	return joined
}
