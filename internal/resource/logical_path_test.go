package resource

import (
	"reflect"
	"testing"
)

func TestValidateLogicalPath(t *testing.T) {
	valid := []string{
		"/",
		"/items",
		"/items/",
		"/alpha/beta",
		"/alpha/beta/",
	}
	for _, path := range valid {
		if err := ValidateLogicalPath(path); err != nil {
			t.Fatalf("expected path %q to be valid, got error: %v", path, err)
		}
	}

	invalid := []string{
		"",
		"items",
		"/items//",
		"/items//foo",
		"/items/../foo",
		"/_/items",
		"/items/_",
		`/items\foo`,
	}
	for _, path := range invalid {
		if err := ValidateLogicalPath(path); err == nil {
			t.Fatalf("expected path %q to be invalid", path)
		}
	}
}

func TestValidateMetadataPath(t *testing.T) {
	valid := []string{
		"/",
		"/items",
		"/items/",
		"/_/items",
		"/items/_",
		"/admin/realms/_/clients",
		"/admin/realms/_/clients/",
	}
	for _, path := range valid {
		if err := ValidateMetadataPath(path); err != nil {
			t.Fatalf("expected path %q to be valid, got error: %v", path, err)
		}
	}

	invalid := []string{
		"",
		"items",
		"/items//",
		"/items//foo",
		"/items/../foo",
		`/items\foo`,
	}
	for _, path := range invalid {
		if err := ValidateMetadataPath(path); err == nil {
			t.Fatalf("expected path %q to be invalid", path)
		}
	}
}

func TestSplitPathSegments(t *testing.T) {
	cases := []struct {
		name string
		path string
		want []string
	}{
		{name: "root", path: "/", want: nil},
		{name: "empty", path: "   ", want: nil},
		{name: "trimmed", path: " /alpha//beta/ ", want: []string{"alpha", "beta"}},
		{name: "spaces", path: "/alpha/ beta / /gamma/", want: []string{"alpha", "beta", "gamma"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitPathSegments(tc.path)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitPathSegments(%q) = %#v, want %#v", tc.path, got, tc.want)
			}
		})
	}
}

func TestPathWildcardVariants(t *testing.T) {
	if got := PathWildcardVariants(nil); got != nil {
		t.Fatalf("expected nil for empty segments, got %#v", got)
	}

	segments := []string{"a", "b"}
	want := [][]string{
		{"a", "b"},
		{"a", "_"},
		{"_", "b"},
		{"_", "_"},
	}
	got := PathWildcardVariants(segments)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PathWildcardVariants(%v) = %#v, want %#v", segments, got, want)
	}
}
