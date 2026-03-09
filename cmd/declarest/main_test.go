package main

import (
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestContextNameFromArgs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "long flag separated",
			args: []string{"--context", "dev"},
			want: "dev",
		},
		{
			name: "short flag separated",
			args: []string{"resource", "list", "-c", "prod"},
			want: "prod",
		},
		{
			name: "long flag equals",
			args: []string{"--context=stage"},
			want: "stage",
		},
		{
			name: "missing context value returns empty",
			args: []string{"resource", "list", "--context"},
			want: "",
		},
		{
			name: "context flag absent",
			args: []string{"resource", "list"},
			want: "",
		},
		{
			name: "context check positional context",
			args: []string{"context", "check", "prod"},
			want: "prod",
		},
		{
			name: "context init positional context",
			args: []string{"context", "init", "prod"},
			want: "prod",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := contextNameFromArgs(testCase.args)
			if got != testCase.want {
				t.Fatalf("contextNameFromArgs() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestIsHelpInvocation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "no args defaults to help",
			args: nil,
			want: true,
		},
		{
			name: "short help flag",
			args: []string{"-h"},
			want: true,
		},
		{
			name: "long help flag",
			args: []string{"--help"},
			want: true,
		},
		{
			name: "help command",
			args: []string{"help", "resource"},
			want: true,
		},
		{
			name: "help token as positional argument is not help invocation",
			args: []string{"context", "use", "help"},
			want: false,
		},
		{
			name: "nested command help flag",
			args: []string{"resource", "save", "--help"},
			want: true,
		},
		{
			name: "help token after double dash ignored",
			args: []string{"resource", "save", "--", "--help"},
			want: false,
		},
		{
			name: "regular command invocation",
			args: []string{"resource", "save", "/customers/acme"},
			want: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := isHelpInvocation(testCase.args)
			if got != testCase.want {
				t.Fatalf("isHelpInvocation() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestIsCompletionInvocation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "empty args",
			args: nil,
			want: false,
		},
		{
			name: "completion command",
			args: []string{"completion"},
			want: true,
		},
		{
			name: "completion subcommand",
			args: []string{"completion", "bash"},
			want: true,
		},
		{
			name: "hidden complete command",
			args: []string{"__complete", "resource", "s"},
			want: true,
		},
		{
			name: "hidden complete no desc command",
			args: []string{"__completeNoDesc", "resource", "s"},
			want: true,
		},
		{
			name: "completion token as positional argument",
			args: []string{"context", "use", "completion"},
			want: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := isCompletionInvocation(testCase.args)
			if got != testCase.want {
				t.Fatalf("isCompletionInvocation() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestShouldSkipContextBootstrap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "help path",
			args: []string{"resource", "save", "--help"},
			want: true,
		},
		{
			name: "completion path",
			args: []string{"completion", "bash"},
			want: true,
		},
		{
			name: "shell completion for runtime command requires bootstrap",
			args: []string{"__complete", "resource", "get", "/ad"},
			want: false,
		},
		{
			name: "shell completion no desc for runtime command requires bootstrap",
			args: []string{"__completeNoDesc", "resource", "get", "/ad"},
			want: false,
		},
		{
			name: "shell completion for command group skips bootstrap",
			args: []string{"__complete", "resource", "g"},
			want: true,
		},
		{
			name: "shell completion for completion command skips bootstrap",
			args: []string{"__complete", "completion", "b"},
			want: true,
		},
		{
			name: "partial command path",
			args: []string{"resource"},
			want: true,
		},
		{
			name: "normal command path",
			args: []string{"resource", "save", "/customers/acme"},
			want: false,
		},
		{
			name: "version command does not require context bootstrap",
			args: []string{"version"},
			want: true,
		},
		{
			name: "context create command does not require context bootstrap",
			args: []string{"context", "create"},
			want: true,
		},
		{
			name: "context list command does not require context bootstrap",
			args: []string{"context", "list"},
			want: true,
		},
		{
			name: "context print-template command does not require context bootstrap",
			args: []string{"context", "print-template"},
			want: true,
		},
		{
			name: "context check command requires context bootstrap",
			args: []string{"context", "check"},
			want: false,
		},
		{
			name: "context init command requires context bootstrap",
			args: []string{"context", "init"},
			want: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got := shouldSkipContextBootstrap(testCase.args)
			if got != testCase.want {
				t.Fatalf("shouldSkipContextBootstrap() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestRequiresContextBootstrap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		commandPath string
		want        bool
	}{
		{
			name:        "resource commands require context",
			commandPath: "declarest resource list",
			want:        true,
		},
		{
			name:        "metadata commands require context",
			commandPath: "declarest metadata resolve",
			want:        true,
		},
		{
			name:        "repository commands require context",
			commandPath: "declarest repository status",
			want:        true,
		},
		{
			name:        "secret commands require context",
			commandPath: "declarest secret resolve",
			want:        true,
		},
		{
			name:        "context check requires context",
			commandPath: "declarest context check",
			want:        true,
		},
		{
			name:        "context init requires context",
			commandPath: "declarest context init",
			want:        true,
		},
		{
			name:        "version does not require context",
			commandPath: "declarest version",
			want:        false,
		},
		{
			name:        "context list does not require context",
			commandPath: "declarest context list",
			want:        false,
		},
		{
			name:        "context print-template does not require context",
			commandPath: "declarest context print-template",
			want:        false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := requiresContextBootstrap(testCase.commandPath)
			if got != testCase.want {
				t.Fatalf("requiresContextBootstrap() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestIsHelpFallbackInvocation(t *testing.T) {
	testCases := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "partial command",
			args: []string{"resource"},
			want: true,
		},
		{
			name: "partial command with global flag",
			args: []string{"--output", "json", "resource"},
			want: true,
		},
		{
			name: "unknown command",
			args: []string{"unknown-command"},
			want: true,
		},
		{
			name: "runnable command",
			args: []string{"resource", "list", "/"},
			want: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got := isHelpFallbackInvocation(testCase.args)
			if got != testCase.want {
				t.Fatalf("isHelpFallbackInvocation() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "plain error", err: errors.New("boom"), want: 1},
		{name: "validation", err: faults.NewTypedError(faults.ValidationError, "invalid", nil), want: 2},
		{name: "not found", err: faults.NewTypedError(faults.NotFoundError, "missing", nil), want: 3},
		{name: "auth", err: faults.NewTypedError(faults.AuthError, "auth", nil), want: 4},
		{name: "conflict", err: faults.NewTypedError(faults.ConflictError, "conflict", nil), want: 5},
		{name: "transport", err: faults.NewTypedError(faults.TransportError, "net", nil), want: 6},
		{name: "internal", err: faults.NewTypedError(faults.InternalError, "internal", nil), want: 1},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCodeForError(testCase.err); got != testCase.want {
				t.Fatalf("exitCodeForError(%v) = %d, want %d", testCase.err, got, testCase.want)
			}
		})
	}
}
