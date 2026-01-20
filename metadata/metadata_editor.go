package metadata

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func SetMetadataAttribute(meta map[string]any, attribute string, value any) (bool, error) {
	if meta == nil {
		return false, errors.New("metadata map is nil")
	}
	parts, err := splitAttributePath(attribute)
	if err != nil {
		return false, err
	}

	current := meta
	for idx := 0; idx < len(parts)-1; idx++ {
		part := parts[idx]
		next, ok := current[part]
		if !ok || next == nil {
			child := map[string]any{}
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return false, fmt.Errorf("attribute %q conflicts with non-object value at %q", attribute, strings.Join(parts[:idx+1], "."))
		}
		current = child
	}

	last := parts[len(parts)-1]
	if existing, ok := current[last]; ok {
		if items, ok := asArray(existing); ok && !isArray(value) {
			if contains(items, value) {
				return false, nil
			}
			current[last] = append(items, value)
			return true, nil
		}
		if reflect.DeepEqual(existing, value) {
			return false, nil
		}
	}

	current[last] = value
	return true, nil
}

func UnsetMetadataAttribute(meta map[string]any, attribute string, value any) (bool, error) {
	if meta == nil {
		return false, errors.New("metadata map is nil")
	}
	parts, err := splitAttributePath(attribute)
	if err != nil {
		return false, err
	}

	parent := meta
	for idx := 0; idx < len(parts)-1; idx++ {
		part := parts[idx]
		next, ok := parent[part]
		if !ok || next == nil {
			return false, fmt.Errorf("attribute %q not found", attribute)
		}
		child, ok := next.(map[string]any)
		if !ok {
			return false, fmt.Errorf("attribute %q conflicts with non-object value at %q", attribute, strings.Join(parts[:idx+1], "."))
		}
		parent = child
	}

	last := parts[len(parts)-1]
	existing, ok := parent[last]
	if !ok {
		return false, fmt.Errorf("attribute %q not found", attribute)
	}

	if items, ok := asArray(existing); ok {
		if isArray(value) {
			if !reflect.DeepEqual(existing, value) {
				return false, fmt.Errorf("attribute %q does not match the provided value", attribute)
			}
			delete(parent, last)
			cleanupEmptyMaps(meta, parts[:len(parts)-1])
			return true, nil
		}
		filtered := make([]any, 0, len(items))
		removed := false
		for _, item := range items {
			if reflect.DeepEqual(item, value) {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		if !removed {
			return false, fmt.Errorf("attribute %q does not include the provided value", attribute)
		}
		if len(filtered) == 0 {
			delete(parent, last)
			cleanupEmptyMaps(meta, parts[:len(parts)-1])
			return true, nil
		}
		parent[last] = filtered
		return true, nil
	}

	if !reflect.DeepEqual(existing, value) {
		return false, fmt.Errorf("attribute %q does not match the provided value", attribute)
	}

	delete(parent, last)
	cleanupEmptyMaps(meta, parts[:len(parts)-1])
	return true, nil
}

func DeleteMetadataAttribute(meta map[string]any, attribute string) (bool, error) {
	if meta == nil {
		return false, errors.New("metadata map is nil")
	}
	parts, err := splitAttributePath(attribute)
	if err != nil {
		return false, err
	}

	parent := meta
	for idx := 0; idx < len(parts)-1; idx++ {
		part := parts[idx]
		next, ok := parent[part]
		if !ok || next == nil {
			return false, fmt.Errorf("attribute %q not found", attribute)
		}
		child, ok := next.(map[string]any)
		if !ok {
			return false, fmt.Errorf("attribute %q conflicts with non-object value at %q", attribute, strings.Join(parts[:idx+1], "."))
		}
		parent = child
	}

	last := parts[len(parts)-1]
	if _, ok := parent[last]; !ok {
		return false, fmt.Errorf("attribute %q not found", attribute)
	}

	delete(parent, last)
	cleanupEmptyMaps(meta, parts[:len(parts)-1])
	return true, nil
}

func splitAttributePath(attribute string) ([]string, error) {
	trimmed := strings.TrimSpace(attribute)
	if trimmed == "" {
		return nil, errors.New("attribute is required")
	}
	parts := strings.Split(trimmed, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("invalid attribute path %q", attribute)
		}
	}
	return parts, nil
}

func cleanupEmptyMaps(root map[string]any, path []string) {
	if len(path) == 0 {
		return
	}
	type frame struct {
		parent map[string]any
		key    string
	}

	var stack []frame
	current := root
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			return
		}
		stack = append(stack, frame{parent: current, key: key})
		current = next
	}

	for idx := len(stack) - 1; idx >= 0; idx-- {
		entry := stack[idx]
		child, ok := entry.parent[entry.key].(map[string]any)
		if !ok || len(child) != 0 {
			return
		}
		delete(entry.parent, entry.key)
	}
}

func asArray(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, len(typed))
		for idx := range typed {
			out[idx] = typed[idx]
		}
		return out, true
	default:
		return nil, false
	}
}

func isArray(value any) bool {
	_, ok := asArray(value)
	return ok
}

func contains(items []any, value any) bool {
	for _, item := range items {
		if reflect.DeepEqual(item, value) {
			return true
		}
	}
	return false
}
