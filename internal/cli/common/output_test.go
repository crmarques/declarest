package common

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteOutputSuppressesNilPayload(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	var value any
	if err := WriteOutput(command, OutputJSON, value, nil); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty output for nil payload, got %q", got)
	}
}

func TestWriteOutputRendersNonNilPayload(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	if err := WriteOutput(command, OutputJSON, map[string]any{"ok": true}, nil); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected non-empty output for non-nil payload")
	}
}

func TestValidateOutputFormatForCommandPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		format  string
		wantErr bool
	}{
		{name: "structured command json", path: "declarest resource get", format: OutputJSON, wantErr: false},
		{name: "text only command auto", path: "declarest secret get", format: OutputAuto, wantErr: false},
		{name: "text only command text", path: "declarest secret get", format: OutputText, wantErr: false},
		{name: "text only command json rejected", path: "declarest secret get", format: OutputJSON, wantErr: true},
		{name: "yaml default command yaml", path: "declarest config show", format: OutputYAML, wantErr: false},
		{name: "yaml default command text", path: "declarest config show", format: OutputText, wantErr: false},
		{name: "yaml default command json rejected", path: "declarest config show", format: OutputJSON, wantErr: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateOutputFormatForCommandPath(testCase.path, testCase.format)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateOutputFormatForCommandPath(%q, %q) error=%v, wantErr=%t", testCase.path, testCase.format, err, testCase.wantErr)
			}
		})
	}
}
