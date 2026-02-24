package metadata

import (
	"errors"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestParsePathDescriptorParityCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		selector     string
		segments     []string
		collection   bool
		selectorMode bool
	}{
		{
			name:         "root_collection",
			input:        "/",
			selector:     "/",
			segments:     nil,
			collection:   true,
			selectorMode: true,
		},
		{
			name:         "resource_path",
			input:        "/admin/realms/master",
			selector:     "/admin/realms/master",
			segments:     []string{"admin", "realms", "master"},
			collection:   false,
			selectorMode: false,
		},
		{
			name:         "collection_trailing_slash",
			input:        "/admin/realms/",
			selector:     "/admin/realms",
			segments:     []string{"admin", "realms"},
			collection:   true,
			selectorMode: true,
		},
		{
			name:         "intermediary_placeholder",
			input:        "/admin/realms/_/clients",
			selector:     "/admin/realms/_/clients",
			segments:     []string{"admin", "realms", "_", "clients"},
			collection:   true,
			selectorMode: true,
		},
		{
			name:         "trailing_placeholder_marker",
			input:        "/admin/realms/_/clients/_",
			selector:     "/admin/realms/_/clients",
			segments:     []string{"admin", "realms", "_", "clients"},
			collection:   true,
			selectorMode: true,
		},
		{
			name:         "wildcard_segment",
			input:        "/customers/*/items",
			selector:     "/customers/*/items",
			segments:     []string{"customers", "*", "items"},
			collection:   true,
			selectorMode: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			descriptor, err := ParsePathDescriptor(test.input)
			if err != nil {
				t.Fatalf("ParsePathDescriptor returned error: %v", err)
			}
			if descriptor.Selector != test.selector {
				t.Fatalf("expected selector %q, got %q", test.selector, descriptor.Selector)
			}
			if !reflect.DeepEqual(descriptor.Segments, test.segments) {
				t.Fatalf("expected segments %#v, got %#v", test.segments, descriptor.Segments)
			}
			if descriptor.Collection != test.collection {
				t.Fatalf("expected collection=%t, got %t", test.collection, descriptor.Collection)
			}
			if descriptor.SelectorMode != test.selectorMode {
				t.Fatalf("expected selectorMode=%t, got %t", test.selectorMode, descriptor.SelectorMode)
			}
		})
	}
}

func TestParsePathDescriptorRejectsInvalidPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "relative", input: "admin/realms"},
		{name: "traversal", input: "/admin/../realms"},
		{name: "invalid_wildcard", input: "/admin/[realms"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParsePathDescriptor(test.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var typedErr *faults.TypedError
			if !errors.As(err, &typedErr) {
				t.Fatalf("expected typed error, got %T", err)
			}
			if typedErr.Category != faults.ValidationError {
				t.Fatalf("expected %q category, got %q", faults.ValidationError, typedErr.Category)
			}
		})
	}
}
