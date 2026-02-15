package resource

import (
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestNormalizeLogicalPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{
			name:  "normalizes_clean_absolute_path",
			input: "/customers/acme",
			want:  "/customers/acme",
		},
		{
			name:  "normalizes_redundant_separators",
			input: "/customers//acme/",
			want:  "/customers/acme",
		},
		{
			name:  "normalizes_windows_separators",
			input: "/customers\\acme",
			want:  "/customers/acme",
		},
		{
			name:  "root_path_allowed",
			input: "/",
			want:  "/",
		},
		{
			name:      "rejects_empty",
			input:     "",
			wantError: true,
		},
		{
			name:      "rejects_relative",
			input:     "customers/acme",
			wantError: true,
		},
		{
			name:      "rejects_traversal_segment",
			input:     "/customers/../acme",
			wantError: true,
		},
		{
			name:      "rejects_reserved_metadata_segment",
			input:     "/customers/_/metadata",
			wantError: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeLogicalPath(test.input)
			if test.wantError {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("NormalizeLogicalPath returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var typed *faults.TypedError
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Category != faults.ValidationError {
		t.Fatalf("expected %q category, got %q", faults.ValidationError, typed.Category)
	}
}
