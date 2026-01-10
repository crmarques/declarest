package cmd

import (
	"testing"

	"declarest/internal/openapi"
)

const sampleSpecJSON = `{
  "openapi": "3.0.0",
  "paths": {
    "/fruits": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/fruits/{id}": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/fruits/{id}/details": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    }
  }
}`

func TestRemoteCompletionCollection(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"", "/"},
		{"/", "/"},
		{"items", "/"},
		{"/items", "/"},
		{"/items/", "/items"},
		{"/items/foo", "/items"},
		{"/items/foo/", "/items/foo"},
		{"/items/foo/bar", "/items/foo"},
		{"/items/foo/bar/", "/items/foo/bar"},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			if got := remoteCompletionCollection(tc.prefix); got != tc.want {
				t.Fatalf("remoteCompletionCollection(%q) = %q, want %q", tc.prefix, got, tc.want)
			}
		})
	}
}

func TestSpecPathSuggestionsFilters(t *testing.T) {
	spec := mustParseSpec(t, sampleSpecJSON)
	suggestions := specPathSuggestions(spec, "/fruits")
	expected := []string{
		"/fruits",
		"/fruits/{id}",
		"/fruits/_",
		"/fruits/{id}/details",
		"/fruits/_/details",
	}
	for _, want := range expected {
		if !containsSuggestion(suggestions, want) {
			t.Fatalf("expected suggestion %q not found in %v", want, suggestions)
		}
	}
}

func mustParseSpec(t *testing.T, data string) *openapi.Spec {
	t.Helper()
	spec, err := openapi.ParseSpec([]byte(data))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return spec
}

func containsSuggestion(list []string, target string) bool {
	for _, entry := range list {
		if entry == target {
			return true
		}
	}
	return false
}
