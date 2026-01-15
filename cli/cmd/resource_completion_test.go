package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"declarest/internal/openapi"
	"declarest/internal/reconciler"
	"declarest/internal/repository"
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

func TestCompletionHasResourceEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []pathCompletionEntry
		want    bool
	}{
		{
			name:    "empty",
			entries: nil,
			want:    false,
		},
		{
			name:    "collectionsOnly",
			entries: []pathCompletionEntry{{value: "/admin/"}, {value: "/fruits/"}},
			want:    false,
		},
		{
			name:    "resourceEntry",
			entries: []pathCompletionEntry{{value: "/admin/realms/master"}},
			want:    true,
		},
		{
			name:    "mixedEntries",
			entries: []pathCompletionEntry{{value: "/admin/"}, {value: "/admin/realms/master"}},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := completionHasResourceEntries(tc.entries); got != tc.want {
				t.Fatalf("completionHasResourceEntries() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMetadataChildEntries(t *testing.T) {
	metaDir := t.TempDir()
	metaPath := filepath.Join(metaDir, "admin", "realms", "_", "user-store", "_")
	if err := os.MkdirAll(metaPath, 0o755); err != nil {
		t.Fatalf("mkdir metadata path: %v", err)
	}
	metaFile := filepath.Join(metaPath, "metadata.json")
	if err := os.WriteFile(metaFile, []byte(`{
  "resourceInfo": {
    "collectionPath": "/admin/realms/{{.realm}}/user-store/"
  }
}`), 0o644); err != nil {
		t.Fatalf("write metadata file: %v", err)
	}

	recon := &reconciler.DefaultReconciler{
		ResourceRecordProvider: repository.NewDefaultResourceRecordProvider(metaDir, nil),
	}

	expect := []string{"/admin/realms/publico/user-store/"}
	info := newCompletionPrefixInfo("/admin/realms/publico/")
	entries, ok := metadataChildEntries(recon, info)
	if !ok {
		t.Fatalf("expected metadata entries for /admin/realms/publico/")
	}
	if got := entriesValues(entries); !reflect.DeepEqual(got, expect) {
		t.Fatalf("metadata entries = %v, want %v", got, expect)
	}

	info = newCompletionPrefixInfo("/admin/realms/publico/u")
	entries, ok = metadataChildEntries(recon, info)
	if !ok {
		t.Fatalf("expected metadata entries for partial prefix /admin/realms/publico/u")
	}
	if got := entriesValues(entries); !reflect.DeepEqual(got, expect) {
		t.Fatalf("metadata entries for partial prefix = %v, want %v", got, expect)
	}
}

func TestMetadataChildEntriesTemplateRepo(t *testing.T) {
	metaDir := filepath.Join("..", "..", "tests", "managed-server", "keycloak", "templates", "repo")
	recon := &reconciler.DefaultReconciler{
		ResourceRecordProvider: repository.NewDefaultResourceRecordProvider(metaDir, nil),
	}

	info := newCompletionPrefixInfo("/admin/realms/publico/")
	entries, ok := metadataChildEntries(recon, info)
	if !ok {
		t.Fatalf("expected metadata entries for template repo")
	}

	found := false
	for _, entry := range entries {
		if entry.value == "/admin/realms/publico/user-store/" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("metadata entries missing user-store: %v", entries)
	}
}
