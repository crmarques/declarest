package common

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newCommandWithStdin(input string) *cobra.Command {
	command := &cobra.Command{}
	command.SetIn(strings.NewReader(input))
	return command
}

func TestReadInputWithFileDashReadsStdin(t *testing.T) {
	command := newCommandWithStdin("  {\"name\":\"value\"}  ")

	data, err := ReadInput(command, InputFlags{File: stdinFileIndicator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(string(data)) != "{\"name\":\"value\"}" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
}

func TestReadInputWithFileDashEmptyInputReportsRequiredError(t *testing.T) {
	command := newCommandWithStdin("   \n")

	_, err := ReadInput(command, InputFlags{File: stdinFileIndicator})
	if err == nil {
		t.Fatalf("expected error for empty stdin")
	}
	if err.Error() != MissingInputMessage {
		t.Fatalf("expected message %q, got %q", MissingInputMessage, err.Error())
	}
}

func TestReadOptionalInputWithFileDashEmptyInputReturnsNil(t *testing.T) {
	command := newCommandWithStdin("\n\n")

	data, err := ReadOptionalInput(command, InputFlags{File: stdinFileIndicator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got %q", string(data))
	}
}
