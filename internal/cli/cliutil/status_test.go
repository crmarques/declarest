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
