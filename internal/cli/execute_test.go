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
		{path: "declarest repo status", want: false},
	}

	for _, testCase := range testCases {
		if got := commandPathSupportsExecutionStatus(testCase.path); got != testCase.want {
			t.Fatalf("commandPathSupportsExecutionStatus(%q) = %t, want %t", testCase.path, got, testCase.want)
		}
	}
}

func TestShouldSuppressColor(t *testing.T) {
	t.Run("no color env", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if !shouldSuppressColor([]string{"resource", "get", "/"}) {
			t.Fatal("expected color suppression when NO_COLOR is set")
		}
	})

	t.Run("flag parsing", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if !shouldSuppressColor([]string{"resource", "get", "/", "--no-color"}) {
			t.Fatal("expected color suppression for --no-color")
		}
		if shouldSuppressColor([]string{"resource", "get", "/", "--no-color=false"}) {
			t.Fatal("expected color enabled when --no-color=false")
		}
	})
}

func TestShouldEmitExecutionStatus(t *testing.T) {
	t.Parallel()

	buildCommandPath := func(names ...string) *cobra.Command {
		root := &cobra.Command{Use: "declarest"}
		current := root
		for _, name := range names {
			next := &cobra.Command{Use: name}
			current.AddCommand(next)
			current = next
		}
		return current
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
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			command := buildCommandPath("resource", "save")
			if testCase.name == "read command" {
				command = buildCommandPath("resource", "get")
			}
			got := shouldEmitExecutionStatus(testCase.args, command)
			if got != testCase.want {
				t.Fatalf("shouldEmitExecutionStatus(%v) = %t, want %t", testCase.args, got, testCase.want)
			}
		})
	}
}
