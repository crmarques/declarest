package common

import (
	"os"
	"path/filepath"
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

	data, err := ReadInput(command, InputFlags{Payload: stdinFileIndicator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(string(data)) != "{\"name\":\"value\"}" {
		t.Fatalf("unexpected payload: %q", string(data))
	}
}

func TestReadInputWithFileDashEmptyInputReportsRequiredError(t *testing.T) {
	command := newCommandWithStdin("   \n")

	_, err := ReadInput(command, InputFlags{Payload: stdinFileIndicator})
	if err == nil {
		t.Fatalf("expected error for empty stdin")
	}
	if err.Error() != MissingInputMessage {
		t.Fatalf("expected message %q, got %q", MissingInputMessage, err.Error())
	}
}

func TestReadOptionalInputWithFileDashEmptyInputReturnsNil(t *testing.T) {
	command := newCommandWithStdin("\n\n")

	data, err := ReadOptionalInput(command, InputFlags{Payload: stdinFileIndicator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got %q", string(data))
	}
}

func TestReadInputRejectsOversizedStdin(t *testing.T) {
	command := newCommandWithStdin(strings.Repeat("a", maxInputBytes+1))

	_, err := ReadInput(command, InputFlags{Payload: stdinFileIndicator})
	if err == nil {
		t.Fatal("expected oversized stdin error")
	}
	if !strings.Contains(err.Error(), "maximum supported size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadInputRejectsOversizedFile(t *testing.T) {
	command := &cobra.Command{}
	path := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", maxInputBytes+1)), 0o600); err != nil {
		t.Fatalf("failed to write oversized file: %v", err)
	}

	_, err := ReadInput(command, InputFlags{Payload: path})
	if err == nil {
		t.Fatal("expected oversized file error")
	}
	if !strings.Contains(err.Error(), "maximum supported size") {
		t.Fatalf("unexpected error: %v", err)
	}
}
