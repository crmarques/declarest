package resource

import "testing"

func TestBuildJSONPatchOperations(t *testing.T) {
	from := map[string]any{
		"a": "same",
		"b": "old",
		"c": map[string]any{
			"d": "remove",
		},
	}
	to := map[string]any{
		"a": "same",
		"b": "new",
		"c": map[string]any{
			"e": "add",
		},
		"f": "added",
	}

	patch := BuildJSONPatch(from, to)

	assertPatchOp(t, patch, "replace", "/b")
	assertPatchOp(t, patch, "remove", "/c/d")
	assertPatchOp(t, patch, "add", "/c/e")
	assertPatchOp(t, patch, "add", "/f")
}

func TestBuildJSONPatchReplacesArrays(t *testing.T) {
	from := []any{"a", "b"}
	to := []any{"a", "c"}

	patch := BuildJSONPatch(from, to)
	if len(patch) != 1 {
		t.Fatalf("expected one patch op, got %#v", patch)
	}
	if patch[0].Op != "replace" || patch[0].Path != "/" {
		t.Fatalf("expected replace /, got %#v", patch[0])
	}
}

func TestBuildJSONPatchEscapesJSONPointer(t *testing.T) {
	from := map[string]any{"a/b": "old", "a~b": "same"}
	to := map[string]any{"a/b": "new", "a~b": "same"}

	patch := BuildJSONPatch(from, to)
	assertPatchOp(t, patch, "replace", "/a~1b")
}

func assertPatchOp(t *testing.T, patch ResourcePatch, op, path string) {
	t.Helper()
	for _, entry := range patch {
		if entry.Op == op && entry.Path == path {
			return
		}
	}
	t.Fatalf("expected patch op %s %s, got %#v", op, path, patch)
}
