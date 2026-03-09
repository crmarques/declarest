package cliutil

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
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
		{name: "yaml default command yaml", path: "declarest context show", format: OutputYAML, wantErr: false},
		{name: "yaml default command text", path: "declarest context show", format: OutputText, wantErr: false},
		{name: "yaml default command json rejected", path: "declarest context show", format: OutputJSON, wantErr: true},
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

func TestWriteOutputAutoWritesBinaryBytesWithoutNewline(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	if err := WriteOutput(command, OutputAuto, resource.BinaryValue{Bytes: []byte("abc")}, nil); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	if got := stdout.String(); got != "abc" {
		t.Fatalf("expected raw binary output, got %q", got)
	}
}

func TestWriteOutputAutoUsesJSONForStructuredValuesWhenStdoutIsNotTTY(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	if err := WriteOutput(command, OutputAuto, map[string]any{"ok": true}, func(w io.Writer, value map[string]any) error {
		_, err := io.WriteString(w, "text\n")
		return err
	}); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "\"ok\": true") {
		t.Fatalf("expected structured auto output to default to json, got %q", output)
	}
	if strings.Contains(output, "text") {
		t.Fatalf("expected json auto output instead of text renderer, got %q", output)
	}
}

func TestWriteOutputJSONWrapsBinaryPayload(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	if err := WriteOutput(command, OutputJSON, resource.BinaryValue{Bytes: []byte("abc")}, nil); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "\"encoding\": \"base64\"") {
		t.Fatalf("expected base64 encoding wrapper, got %q", output)
	}
	if !strings.Contains(output, "\"data\": \"YWJj\"") {
		t.Fatalf("expected base64 payload data, got %q", output)
	}
}

func TestWriteOutputAutoUsesJSONForBinaryCollectionsWhenStdoutIsNotTTY(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	stdout := &bytes.Buffer{}
	command.SetOut(stdout)

	if err := WriteOutput(command, OutputAuto, []any{resource.BinaryValue{Bytes: []byte("abc")}}, nil); err != nil {
		t.Fatalf("WriteOutput returned error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "\"encoding\": \"base64\"") {
		t.Fatalf("expected binary collection auto output to default to json, got %q", output)
	}
}
