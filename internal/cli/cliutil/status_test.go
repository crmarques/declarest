package cliutil

import (
	"bytes"
	"testing"
)

func TestWriteStatusLineUsesUppercaseBracketedLabels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		status  string
		message string
		want    string
	}{
		{
			name:    "warning",
			status:  "warning",
			message: "managed-server.http.base-url uses plain HTTP",
			want:    "[WARNING] managed-server.http.base-url uses plain HTTP\n",
		},
		{
			name:    "ok",
			status:  "ok",
			message: "command executed successfully.",
			want:    "[OK] command executed successfully.\n",
		},
		{
			name:    "no message",
			status:  "warning",
			message: " ",
			want:    "[WARNING]\n",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			WriteStatusLine(&buf, testCase.status, testCase.message)
			if got := buf.String(); got != testCase.want {
				t.Fatalf("WriteStatusLine(%q, %q) = %q, want %q", testCase.status, testCase.message, got, testCase.want)
			}
		})
	}
}

func TestShouldIgnoreWarnings(t *testing.T) {
	t.Run("flag parsing", func(t *testing.T) {
		if !ShouldIgnoreWarnings([]string{"resource", "get", "/", "--ignore-warnings"}) {
			t.Fatal("expected warning suppression for --ignore-warnings")
		}
		if ShouldIgnoreWarnings([]string{"resource", "get", "/", "--ignore-warnings=false"}) {
			t.Fatal("expected warnings enabled for --ignore-warnings=false")
		}
	})

	t.Run("env default", func(t *testing.T) {
		t.Setenv(GlobalEnvIgnoreWarnings, "true")
		if !ShouldIgnoreWarnings([]string{"resource", "get", "/"}) {
			t.Fatal("expected warning suppression when DECLAREST_IGNORE_WARNINGS is set")
		}
		if ShouldIgnoreWarnings([]string{"resource", "get", "/", "--ignore-warnings=false"}) {
			t.Fatal("expected explicit flag to override DECLAREST_IGNORE_WARNINGS")
		}
	})
}
