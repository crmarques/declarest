package cmd

import (
	"reflect"
	"testing"

	"declarest/internal/openapi"
	"declarest/internal/reconciler"
	"declarest/internal/resource"
)

const sampleSpecJSON = `{
  "openapi": "3.0.0",
  "paths": {
    "/admin": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/admin/realms": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/admin/realms/{id}": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/admin/realms/{id}/clients": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/admin/realms/{id}/clients/{client}": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
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
    },
    "/vegetables": {
      "get": {
        "responses": {
          "200": {}
        }
      }
    },
    "/vegetables/{id}": {
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

func TestSpecChildEntries(t *testing.T) {
	spec := mustParseSpec(t, sampleSpecJSON)

	rootEntries := specChildEntries(spec, "/")
	expectedRoot := []string{"/admin/", "/fruits/", "/vegetables/"}
	if got := entriesValues(rootEntries); !reflect.DeepEqual(got, expectedRoot) {
		t.Fatalf("root child entries = %v, want %v", got, expectedRoot)
	}

	if got := entriesValues(specChildEntries(spec, "/admin/re")); !reflect.DeepEqual(got, []string{"/admin/realms/"}) {
		t.Fatalf("unexpected mis-completion for /admin/re: %v", got)
	}

	if got := entriesValues(specChildEntries(spec, "/a")); !reflect.DeepEqual(got, []string{"/admin/"}) {
		t.Fatalf("unexpected children for /a: %v", got)
	}
	if got := entriesValues(specChildEntries(spec, "/fruits")); !reflect.DeepEqual(got, []string{"/fruits/"}) {
		t.Fatalf("unexpected children for /fruits: %v", got)
	}
	if got := entriesValues(specChildEntries(spec, "/admin/realms/123/")); !reflect.DeepEqual(got, []string{"/admin/realms/123/clients/"}) {
		t.Fatalf("unexpected children for /admin/realms/123/: %v", got)
	}
}

func TestSpecCollectionPath(t *testing.T) {
	spec := mustParseSpec(t, sampleSpecJSON)

	tests := []struct {
		path string
		want bool
	}{
		{"/admin/", false},
		{"/admin/realms/", true},
		{"/admin/realms/123/", false},
		{"/admin/realms/123/clients/", true},
		{"/fruits/", true},
		{"/fruits/123/", false},
		{"/vegetables/", true},
		{"/unknown/", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := specCollectionPath(spec, tc.path); got != tc.want {
				t.Fatalf("specCollectionPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestSpecHasPrefix(t *testing.T) {
	spec := mustParseSpec(t, sampleSpecJSON)

	tests := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/admin/", true},
		{"/admin/realms/123/", true},
		{"/unknown/", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := specHasPrefix(spec, tc.path); got != tc.want {
				t.Fatalf("specHasPrefix(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
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

func entriesValues(entries []pathCompletionEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.value)
	}
	return values
}

func TestCollectionCompletionEntry(t *testing.T) {
	info := &resource.ResourceInfoMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "clientId",
	}
	entry, ok := collectionCompletionEntry("/admin/realms/master/clients", "abcd", "abcd", "clientA", info)
	if !ok {
		t.Fatalf("expected completion entry")
	}
	if entry.value != "/admin/realms/master/clients/abcd" {
		t.Fatalf("entry value = %q, want %q", entry.value, "/admin/realms/master/clients/abcd")
	}
	if entry.description != "clientA" {
		t.Fatalf("entry description = %q, want %q", entry.description, "clientA")
	}

	same := &resource.ResourceInfoMetadata{
		IDFromAttribute:    "realm",
		AliasFromAttribute: "realm",
	}
	entry, ok = collectionCompletionEntry("/admin/realms", "master", "master", "master", same)
	if !ok {
		t.Fatalf("expected completion entry for same-attr case")
	}
	if entry.description != "" {
		t.Fatalf("entry description = %q, want %q", entry.description, "")
	}
}

func TestCompletionDescription(t *testing.T) {
	tests := []struct {
		name       string
		entry      reconciler.RemoteResourceEntry
		remotePath string
		wantDesc   string
	}{
		{
			name:       "aliasEqualsID",
			entry:      reconciler.RemoteResourceEntry{ID: "master", Alias: "master"},
			remotePath: "/admin/realms/master",
			wantDesc:   "",
		},
		{
			name:       "aliasDiffersFromID",
			entry:      reconciler.RemoteResourceEntry{ID: "123", Alias: "alpha"},
			remotePath: "/items/alpha",
			wantDesc:   "123",
		},
		{
			name:       "displaySegmentIsID",
			entry:      reconciler.RemoteResourceEntry{ID: "123", Alias: "alpha"},
			remotePath: "/items/123",
			wantDesc:   "alpha",
		},
		{
			name:       "aliasMissing",
			entry:      reconciler.RemoteResourceEntry{ID: "abc"},
			remotePath: "/items/abc",
			wantDesc:   "",
		},
		{
			name:       "fallbackToPath",
			entry:      reconciler.RemoteResourceEntry{},
			remotePath: "/",
			wantDesc:   "",
		},
		{
			name:       "sanitizedValues",
			entry:      reconciler.RemoteResourceEntry{ID: "  id\tvalue\n", Alias: " alias "},
			remotePath: "/items/alias",
			wantDesc:   "id value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			desc := completionDescription(tc.entry, tc.remotePath)
			if desc != tc.wantDesc {
				t.Fatalf("completionDescription(%+v) = %q, want %q", tc.entry, desc, tc.wantDesc)
			}
		})
	}
}
