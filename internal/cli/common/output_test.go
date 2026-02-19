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
