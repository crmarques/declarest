// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestParseRawPathWithOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		input              string
		options            RawPathParseOptions
		wantNormalized     string
		wantSegments       []string
		wantCollectionFlag bool
		wantError          bool
	}{
		{
			name:               "tracks_trailing_collection_marker",
			input:              "/customers/",
			wantNormalized:     "/customers",
			wantSegments:       []string{"customers"},
			wantCollectionFlag: true,
		},
		{
			name:           "preserves_root_without_collection_marker",
			input:          "/",
			wantNormalized: "/",
		},
		{
			name:           "allows_missing_leading_slash_when_configured",
			input:          "customers/acme",
			options:        RawPathParseOptions{AllowMissingLeadingSlash: true},
			wantNormalized: "/customers/acme",
			wantSegments:   []string{"customers", "acme"},
		},
		{
			name:      "rejects_relative_path_by_default",
			input:     "customers/acme",
			wantError: true,
		},
		{
			name:      "rejects_traversal",
			input:     "/customers/../acme",
			wantError: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseRawPathWithOptions(test.input, test.options)
			if test.wantError {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("ParseRawPathWithOptions returned error: %v", err)
			}
			if got.Normalized != test.wantNormalized {
				t.Fatalf("expected normalized %q, got %q", test.wantNormalized, got.Normalized)
			}
			if got.ExplicitCollectionTarget != test.wantCollectionFlag {
				t.Fatalf("expected collection marker=%t, got %t", test.wantCollectionFlag, got.ExplicitCollectionTarget)
			}
			if len(got.Segments) != len(test.wantSegments) {
				t.Fatalf("expected segments %#v, got %#v", test.wantSegments, got.Segments)
			}
			for idx := range got.Segments {
				if got.Segments[idx] != test.wantSegments[idx] {
					t.Fatalf("expected segments %#v, got %#v", test.wantSegments, got.Segments)
				}
			}
		})
	}
}

func TestHasLogicalPathOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		left      string
		right     string
		want      bool
		wantError bool
	}{
		{name: "same_path", left: "/customers/acme", right: "/customers/acme", want: true},
		{name: "parent_child", left: "/customers", right: "/customers/acme", want: true},
		{name: "siblings", left: "/customers/acme", right: "/customers/beta", want: false},
		{name: "root_scope", left: "/", right: "/customers", want: true},
		{name: "rejects_invalid_path", left: "/customers/../acme", right: "/customers", wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := HasLogicalPathOverlap(test.left, test.right)
			if test.wantError {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatalf("HasLogicalPathOverlap returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("HasLogicalPathOverlap(%q, %q) = %v, want %v", test.left, test.right, got, test.want)
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
