package main

import "testing"

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
			args: []string{"config", "use", "help"},
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
			args: []string{"config", "use", "completion"},
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
			name: "partial command path",
			args: []string{"resource"},
			want: true,
		},
		{
			name: "normal command path",
			args: []string{"resource", "save", "/customers/acme"},
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
