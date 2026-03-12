package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
)

func TestShouldSuppressStatusMessage(t *testing.T) {
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

	t.Run("env default", func(t *testing.T) {
		t.Setenv(cliutil.GlobalEnvNoStatus, "true")
		if !shouldSuppressStatusMessage([]string{"resource", "list", "/"}) {
			t.Fatal("expected status suppression when DECLAREST_NO_STATUS is set")
		}
		if shouldSuppressStatusMessage([]string{"resource", "list", "/", "--no-status=false"}) {
			t.Fatal("expected explicit flag to override DECLAREST_NO_STATUS")
		}
	})
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

func TestEmitsExecutionStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		names []string
		mark  bool
		want  bool
	}{
		{names: []string{"resource", "save"}, mark: true, want: true},
		{names: []string{"resource", "apply"}, mark: true, want: true},
		{names: []string{"repository", "commit"}, mark: true, want: true},
		{names: []string{"resource", "get"}, want: false},
		{names: []string{"repository", "status"}, want: false},
	}

	for _, testCase := range testCases {
		root := &cobra.Command{Use: "declarest"}
		current := root
		for _, name := range testCase.names {
			next := &cobra.Command{Use: name}
			current.AddCommand(next)
			current = next
		}
		if testCase.mark {
			commandmeta.MarkEmitsExecutionStatus(current)
		}
		if got := commandmeta.EmitsExecutionStatus(current); got != testCase.want {
			t.Fatalf("EmitsExecutionStatus(%v) = %t, want %t", testCase.names, got, testCase.want)
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
		t.Setenv(cliutil.GlobalEnvNoColorLegacy, "")
		if !shouldSuppressColor([]string{"resource", "get", "/", "--no-color"}) {
			t.Fatal("expected color suppression for --no-color")
		}
		if shouldSuppressColor([]string{"resource", "get", "/", "--no-color=false"}) {
			t.Fatal("expected color enabled when --no-color=false")
		}
	})

	t.Run("env default", func(t *testing.T) {
		t.Setenv(cliutil.GlobalEnvNoColor, "true")
		if !shouldSuppressColor([]string{"resource", "get", "/"}) {
			t.Fatal("expected color suppression when DECLAREST_NO_COLOR is set")
		}
		if shouldSuppressColor([]string{"resource", "get", "/", "--no-color=false"}) {
			t.Fatal("expected explicit flag to override DECLAREST_NO_COLOR")
		}
	})
}

func TestShouldEmitExecutionStatus(t *testing.T) {
	t.Parallel()

	buildCommandPath := func(mark bool, names ...string) *cobra.Command {
		root := &cobra.Command{Use: "declarest"}
		current := root
		for _, name := range names {
			next := &cobra.Command{Use: name}
			current.AddCommand(next)
			current = next
		}
		if mark {
			commandmeta.MarkEmitsExecutionStatus(current)
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
			command := buildCommandPath(true, "resource", "save")
			if testCase.name == "read command" {
				command = buildCommandPath(false, "resource", "get")
			}
			got := shouldEmitExecutionStatus(testCase.args, command)
			if got != testCase.want {
				t.Fatalf("shouldEmitExecutionStatus(%v) = %t, want %t", testCase.args, got, testCase.want)
			}
		})
	}
}
