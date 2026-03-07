package resource

import (
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/spf13/cobra"
)

func TestNormalizeReadSourceSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceFlag     string
		want           string
		wantValidation bool
	}{
		{name: "default_managed_server", want: sourceManagedServer},
		{name: "new_flag_repository", sourceFlag: sourceRepository, want: sourceRepository},
		{name: "new_flag_managed_server", sourceFlag: sourceManagedServer, want: sourceManagedServer},
		{name: "rejects_both_value", sourceFlag: sourceBoth, wantValidation: true},
		{name: "rejects_invalid_value", sourceFlag: "invalid", wantValidation: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeReadSourceSelection(tt.sourceFlag)
			if tt.wantValidation {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if !faults.IsCategory(err, faults.ValidationError) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected source: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDeleteSourceSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceFlag     string
		want           string
		wantValidation bool
	}{
		{name: "default_managed_server", want: sourceManagedServer},
		{name: "new_flag_both", sourceFlag: sourceBoth, want: sourceBoth},
		{name: "rejects_invalid", sourceFlag: "invalid", wantValidation: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeDeleteSourceSelection(tt.sourceFlag)
			if tt.wantValidation {
				if err == nil {
					t.Fatal("expected validation error")
				}
				if !faults.IsCategory(err, faults.ValidationError) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected source: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestParseSkipItemsFlag(t *testing.T) {
	t.Parallel()

	t.Run("unset_flag_returns_nil", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw string
		bindSkipItemsFlag(command, &raw)

		got, err := parseSkipItemsFlag(command, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil items for unset flag, got %#v", got)
		}
	})

	t.Run("parses_trimmed_unique_values", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw string
		bindSkipItemsFlag(command, &raw)
		if err := command.Flags().Set("skip-items", " master, realm1 ,master "); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}

		got, err := parseSkipItemsFlag(command, raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"master", "realm1"}
		if len(got) != len(want) {
			t.Fatalf("unexpected parsed items: got=%#v want=%#v", got, want)
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("unexpected parsed item at %d: got=%q want=%q", idx, got[idx], want[idx])
			}
		}
	})

	t.Run("rejects_empty_item", func(t *testing.T) {
		t.Parallel()

		command := &cobra.Command{Use: "test"}
		var raw string
		bindSkipItemsFlag(command, &raw)
		if err := command.Flags().Set("skip-items", "master,,realm1"); err != nil {
			t.Fatalf("unexpected set error: %v", err)
		}

		_, err := parseSkipItemsFlag(command, raw)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !faults.IsCategory(err, faults.ValidationError) {
			t.Fatalf("expected validation error, got %v", err)
		}
	})
}
