package resource

import (
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestNormalizeReadSourceSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sourceFlag     string
		want           string
		wantValidation bool
	}{
		{name: "default_remote", want: sourceRemoteServer},
		{name: "new_flag_repository", sourceFlag: sourceRepository, want: sourceRepository},
		{name: "new_flag_remote", sourceFlag: sourceRemoteServer, want: sourceRemoteServer},
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
		{name: "default_remote", want: sourceRemoteServer},
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
