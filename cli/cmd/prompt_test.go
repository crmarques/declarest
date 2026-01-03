package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadLineHandlesBackspace(t *testing.T) {
	input := "abc\b\bXY\n"
	out := &bytes.Buffer{}
	prompt := newPrompter(strings.NewReader(input), out)

	got, err := prompt.readLine("Prompt: ")
	if err != nil {
		t.Fatalf("readLine error: %v", err)
	}

	if got != "aXY" {
		t.Fatalf("expected %q, got %q", "aXY", got)
	}
}

func TestReadLineHandlesDelete(t *testing.T) {
	input := "ab\x7fcd\n"
	out := &bytes.Buffer{}
	prompt := newPrompter(strings.NewReader(input), out)

	got, err := prompt.readLine("Prompt: ")
	if err != nil {
		t.Fatalf("readLine error: %v", err)
	}

	if got != "acd" {
		t.Fatalf("expected %q, got %q", "acd", got)
	}
}
