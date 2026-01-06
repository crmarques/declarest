package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestChoiceReturnsDefaultWhenEmpty(t *testing.T) {
	input := "\n"
	out := &bytes.Buffer{}
	prompt := newPrompter(strings.NewReader(input), out)

	value, err := prompt.choice("Sample", []string{"first", "second"}, "first", func(raw string) (string, bool) {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "first":
			return "first", true
		case "second":
			return "second", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("choice error: %v", err)
	}
	if value != "first" {
		t.Fatalf("expected default 'first', got %q", value)
	}
}

func TestChoiceAcceptsNumericSelection(t *testing.T) {
	input := "2\n"
	out := &bytes.Buffer{}
	prompt := newPrompter(strings.NewReader(input), out)

	value, err := prompt.choice("Sample", []string{"first", "second"}, "first", func(raw string) (string, bool) {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "first":
			return "first", true
		case "second":
			return "second", true
		default:
			return "", false
		}
	})
	if err != nil {
		t.Fatalf("choice error: %v", err)
	}
	if value != "second" {
		t.Fatalf("expected 'second', got %q", value)
	}
}

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
