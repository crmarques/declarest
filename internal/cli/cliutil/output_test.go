// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cliutil

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/crmarques/declarest/internal/cli/commandmeta"
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

func TestValidateOutputFormatForCommand(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		command *cobra.Command
		format  string
		wantErr bool
	}{
		{name: "structured command json", command: &cobra.Command{}, format: OutputJSON, wantErr: false},
		{name: "text only command auto", command: markedCommand(commandmeta.MarkTextOnlyOutput), format: OutputAuto, wantErr: false},
		{name: "text only command text", command: markedCommand(commandmeta.MarkTextOnlyOutput), format: OutputText, wantErr: false},
		{name: "text only command json rejected", command: markedCommand(commandmeta.MarkTextOnlyOutput), format: OutputJSON, wantErr: true},
		{name: "yaml default command yaml", command: markedCommand(commandmeta.MarkYAMLDefaultTextOrYAMLOutput), format: OutputYAML, wantErr: false},
		{name: "yaml default command text", command: markedCommand(commandmeta.MarkYAMLDefaultTextOrYAMLOutput), format: OutputText, wantErr: false},
		{name: "yaml default command json rejected", command: markedCommand(commandmeta.MarkYAMLDefaultTextOrYAMLOutput), format: OutputJSON, wantErr: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateOutputFormatForCommand(testCase.command, testCase.format)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateOutputFormatForCommand(%q) error=%v, wantErr=%t", testCase.format, err, testCase.wantErr)
			}
		})
	}
}

func markedCommand(mark func(*cobra.Command)) *cobra.Command {
	command := &cobra.Command{}
	mark(command)
	return command
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
