package orchestrator

import (
	"sort"
	"strings"

	"github.com/crmarques/declarest/resource"
)

func applyFilterPointers(value resource.Value, pointers []string) (resource.Value, error) {
	normalizedPointers, err := normalizePointers(pointers)
	if err != nil {
		return nil, err
	}

	result := any(nil)
	for _, pointer := range normalizedPointers {
		foundValue, found, err := resource.LookupJSONPointer(value, pointer)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if pointer == "" {
			return resource.DeepCopyValue(value), nil
		}

		result, err = resource.SetJSONPointerValue(result, pointer, foundValue)
		if err != nil {
			return nil, err
		}
	}

	if result == nil {
		switch value.(type) {
		case []any:
			return []any{}, nil
		case map[string]any:
			return map[string]any{}, nil
		default:
			return nil, nil
		}
	}

	return result, nil
}

func applySuppressPointers(value resource.Value, pointers []string) (resource.Value, error) {
	normalizedPointers, err := normalizePointers(pointers)
	if err != nil {
		return nil, err
	}

	working := resource.DeepCopyValue(value)
	for _, pointer := range normalizedPointers {
		if pointer == "" {
			return nil, nil
		}

		working, err = resource.DeleteJSONPointerValue(working, pointer)
		if err != nil {
			return nil, err
		}
	}

	return working, nil
}

func normalizePointers(pointers []string) ([]string, error) {
	if len(pointers) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(pointers))
	seen := make(map[string]struct{}, len(pointers))

	for _, rawPointer := range pointers {
		pointer := strings.TrimSpace(rawPointer)
		if _, found := seen[pointer]; found {
			continue
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return nil, err
		}

		seen[pointer] = struct{}{}
		normalized = append(normalized, pointer)
	}

	sort.Strings(normalized)
	return normalized, nil
}
