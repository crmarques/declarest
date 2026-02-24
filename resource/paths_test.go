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

func TestJoinLogicalPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parent    string
		segment   string
		want      string
		wantError bool
	}{
		{name: "joins collection child", parent: "/customers", segment: "acme", want: "/customers/acme"},
		{name: "trims whitespace", parent: "/customers", segment: " acme ", want: "/customers/acme"},
		{name: "normalizes nested segments", parent: "/customers", segment: "acme/dev", want: "/customers/acme/dev"},
		{name: "joins root", parent: "/", segment: "customers", want: "/customers"},
		{name: "rejects empty segment", parent: "/customers", segment: " ", wantError: true},
		{name: "rejects reserved segment", parent: "/customers", segment: "_", wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := JoinLogicalPath(test.parent, test.segment)
			if test.wantError {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("JoinLogicalPath returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestSplitLogicalPathSegments(t *testing.T) {
	t.Parallel()

	if got := SplitLogicalPathSegments("/"); got != nil {
		t.Fatalf("expected nil for root path, got %#v", got)
	}
	if got := SplitLogicalPathSegments("/customers/acme"); len(got) != 2 || got[0] != "customers" || got[1] != "acme" {
		t.Fatalf("unexpected segments: %#v", got)
	}
	if got := SplitLogicalPathSegments("relative"); got != nil {
		t.Fatalf("expected nil for invalid path, got %#v", got)
	}
}

func TestChildSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parent    string
		candidate string
		want      string
		ok        bool
	}{
		{name: "direct child", parent: "/customers", candidate: "/customers/acme", want: "acme", ok: true},
		{name: "nested descendant rejected", parent: "/customers", candidate: "/customers/acme/dev", ok: false},
		{name: "same path rejected", parent: "/customers", candidate: "/customers", ok: false},
		{name: "different branch rejected", parent: "/customers", candidate: "/accounts/acme", ok: false},
		{name: "root direct child", parent: "/", candidate: "/customers", want: "customers", ok: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ChildSegment(test.parent, test.candidate)
			if ok != test.ok {
				t.Fatalf("ChildSegment(%q, %q) ok=%t, want %t", test.parent, test.candidate, ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("ChildSegment(%q, %q) = %q, want %q", test.parent, test.candidate, got, test.want)
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
