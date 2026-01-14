package cmd

import (
	"os"
	"reflect"
	"testing"
)

func TestResolveEditorCommandDefaultsToVi(t *testing.T) {
	origVISUAL := os.Getenv("VISUAL")
	origEDITOR := os.Getenv("EDITOR")
	t.Cleanup(func() {
		_ = os.Setenv("VISUAL", origVISUAL)
		_ = os.Setenv("EDITOR", origEDITOR)
	})
	if err := os.Setenv("VISUAL", "code"); err != nil {
		t.Fatalf("set VISUAL: %v", err)
	}
	if err := os.Setenv("EDITOR", "nano"); err != nil {
		t.Fatalf("set EDITOR: %v", err)
	}

	args, err := resolveEditorCommand("", "")
	if err != nil {
		t.Fatalf("resolveEditorCommand: %v", err)
	}
	if len(args) != 1 || args[0] != "vi" {
		t.Fatalf("expected vi even when VISUAL/EDITOR set, got %v", args)
	}
}

func TestResolveEditorCommandWithOverride(t *testing.T) {
	args, err := resolveEditorCommand("code --wait", "")
	if err != nil {
		t.Fatalf("resolveEditorCommand: %v", err)
	}
	want := []string{"code", "--wait"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("expected %v, got %v", want, args)
	}
}

func TestResolveEditorCommandUsesFallback(t *testing.T) {
	args, err := resolveEditorCommand("", "code --wait")
	if err != nil {
		t.Fatalf("resolveEditorCommand: %v", err)
	}
	want := []string{"code", "--wait"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("expected %v, got %v", want, args)
	}
}
