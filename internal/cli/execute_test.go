package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldSuppressStatusMessage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "default false", args: []string{"resource", "list", "/"}, want: false},
		{name: "long flag", args: []string{"--no-status", "resource", "list", "/"}, want: true},
		{name: "short flag", args: []string{"-n", "resource", "list", "/"}, want: true},
		{name: "flag after positionals", args: []string{"resource", "list", "/", "--no-status"}, want: true},
		{name: "explicit true", args: []string{"--no-status=true", "resource", "list", "/"}, want: true},
		{name: "explicit false", args: []string{"--no-status=false", "resource", "list", "/"}, want: false},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := shouldSuppressStatusMessage(testCase.args)
			if got != testCase.want {
				t.Fatalf("shouldSuppressStatusMessage(%v) = %t, want %t", testCase.args, got, testCase.want)
			}
		})
	}
}

func TestExecutionStatusWriters(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		buffer := &bytes.Buffer{}
		writeExecutionOKStatus(buffer)
		if got, want := buffer.String(), "[OK] command executed successfully.\n"; got != want {
			t.Fatalf("writeExecutionOKStatus() = %q, want %q", got, want)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		buffer := &bytes.Buffer{}
		writeExecutionErrorStatus(buffer, errors.New("resource not found"))
		if got, want := buffer.String(), "[ERROR] command execution failed: resource not found.\n"; got != want {
			t.Fatalf("writeExecutionErrorStatus() = %q, want %q", got, want)
		}
	})
}

func TestCommandPathSupportsExecutionStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path string
		want bool
	}{
		{path: "declarest resource save", want: true},
		{path: "declarest resource apply", want: true},
		{path: "declarest resource create", want: true},
		{path: "declarest resource update", want: true},
		{path: "declarest resource delete", want: true},
		{path: "declarest resource get", want: false},
		{path: "declarest resource list", want: false},
		{path: "declarest resource diff", want: false},
		{path: "declarest ad-hoc get", want: true},
		{path: "declarest ad-hoc post", want: true},
		{path: "declarest ad-hoc delete", want: true},
		{path: "declarest ad-hoc", want: false},
		{path: "declarest repo status", want: false},
	}

	for _, testCase := range testCases {
		if got := commandPathSupportsExecutionStatus(testCase.path); got != testCase.want {
			t.Fatalf("commandPathSupportsExecutionStatus(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

func TestShouldEmitExecutionStatus(t *testing.T) {
	t.Parallel()

	buildSubcommand := func(parentName string, subcommandName string) *cobra.Command {
		root := &cobra.Command{Use: "declarest"}
		parentCommand := &cobra.Command{Use: parentName}
		subcommand := &cobra.Command{Use: subcommandName}
		parentCommand.AddCommand(subcommand)
		root.AddCommand(parentCommand)
		return subcommand
	}

	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "mutation command", args: []string{"resource", "save", "/customers/acme"}, want: true},
		{name: "mutation command no status", args: []string{"resource", "save", "/customers/acme", "--no-status"}, want: false},
		{name: "help invocation", args: []string{"resource", "save", "--help"}, want: false},
		{name: "completion invocation", args: []string{"completion", "bash"}, want: false},
		{name: "read command", args: []string{"resource", "get", "/customers/acme"}, want: false},
		{name: "ad-hoc command", args: []string{"ad-hoc", "get", "/health"}, want: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			command := buildSubcommand("resource", "save")
			if testCase.name == "read command" {
				command = buildSubcommand("resource", "get")
			} else if testCase.name == "ad-hoc command" {
				command = buildSubcommand("ad-hoc", "get")
			}
			got := shouldEmitExecutionStatus(testCase.args, command)
			if got != testCase.want {
				t.Fatalf("shouldEmitExecutionStatus(%v) = %t, want %t", testCase.args, got, testCase.want)
			}
		})
	}
}
