package resource

import (
	"reflect"
	"strings"
)

// BuildJSONPatch creates a JSON Patch-style diff between two values.
// It produces add/remove/replace operations for map fields; arrays and mismatched
// types fall back to a replace of the current path.
func BuildJSONPatch(from, to any) ResourcePatch {
	var patch ResourcePatch
	buildPatch(from, to, "", &patch)
	return patch
}

func buildPatch(from, to any, path string, patch *ResourcePatch) {
	if reflect.DeepEqual(from, to) {
		return
	}

	switch fromMap := toMap(from); {
	case fromMap != nil && toMap(to) != nil:
		toMapVal := toMap(to)
		visited := make(map[string]struct{})
		for key, fromVal := range fromMap {
			visited[key] = struct{}{}
			if toVal, ok := toMapVal[key]; ok {
				buildPatch(fromVal, toVal, joinPath(path, key), patch)
			} else {
				*patch = append(*patch, ResourcePatchOp{Op: "remove", Path: joinPath(path, key)})
			}
		}
		for key, toVal := range toMapVal {
			if _, ok := visited[key]; ok {
				continue
			}
			*patch = append(*patch, ResourcePatchOp{Op: "add", Path: joinPath(path, key), Value: toVal})
		}
	case isSlice(from) && isSlice(to):
		// If arrays differ, replace the whole array path for simplicity.
		*patch = append(*patch, ResourcePatchOp{Op: "replace", Path: pathOrRoot(path), Value: to})
	default:
		*patch = append(*patch, ResourcePatchOp{Op: "replace", Path: pathOrRoot(path), Value: to})
	}
}

func pathOrRoot(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func joinPath(base, segment string) string {
	escaped := escapeJSONPointer(segment)
	if base == "" {
		return "/" + escaped
	}
	return base + "/" + escaped
}

func escapeJSONPointer(seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	return strings.ReplaceAll(seg, "/", "~1")
}

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func isSlice(v any) bool {
	_, ok := v.([]any)
	return ok
}
