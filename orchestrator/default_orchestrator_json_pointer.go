package orchestrator

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func applyFilterPointers(value resource.Value, pointers []string) (resource.Value, error) {
	normalizedPointers, err := normalizePointers(pointers)
	if err != nil {
		return nil, err
	}

	result := any(nil)
	for _, pointer := range normalizedPointers {
		tokens, err := parsePointerTokens(pointer)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			return deepCopyValue(value), nil
		}

		foundValue, found, err := lookupPointerValue(value, tokens)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		result, err = setPointerValue(result, tokens, foundValue)
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

	working := deepCopyValue(value)
	for _, pointer := range normalizedPointers {
		tokens, err := parsePointerTokens(pointer)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			return nil, nil
		}

		working, err = deletePointerValue(working, tokens)
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

	for _, pointer := range pointers {
		value := strings.TrimSpace(pointer)
		if value == "" {
			value = "/"
		}
		if _, found := seen[value]; found {
			continue
		}
		if value != "/" && !strings.HasPrefix(value, "/") {
			return nil, faults.NewTypedError(
				faults.ValidationError,
				fmt.Sprintf("invalid compare pointer %q", pointer),
				nil,
			)
		}

		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	sort.Strings(normalized)
	return normalized, nil
}

func parsePointerTokens(pointer string) ([]string, error) {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" || trimmed == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		return nil, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("invalid compare pointer %q", pointer),
			nil,
		)
	}

	rawTokens := strings.Split(trimmed[1:], "/")
	tokens := make([]string, len(rawTokens))
	for idx, token := range rawTokens {
		unescaped := strings.ReplaceAll(token, "~1", "/")
		unescaped = strings.ReplaceAll(unescaped, "~0", "~")
		tokens[idx] = unescaped
	}
	return tokens, nil
}

func lookupPointerValue(value any, tokens []string) (any, bool, error) {
	current := value
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			item, found := typed[token]
			if !found {
				return nil, false, nil
			}
			current = item
		case []any:
			index, ok := parseArrayIndex(token)
			if !ok || index < 0 || index >= len(typed) {
				return nil, false, nil
			}
			current = typed[index]
		default:
			return nil, false, nil
		}
	}

	return deepCopyValue(current), true, nil
}

func setPointerValue(root any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return deepCopyValue(value), nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseArrayIndex(head); isIndex {
		var items []any
		switch typed := root.(type) {
		case nil:
			items = make([]any, index+1)
		case []any:
			items = typed
			if len(items) <= index {
				grown := make([]any, index+1)
				copy(grown, items)
				items = grown
			}
		default:
			return nil, faults.NewTypedError(
				faults.ValidationError,
				"compare pointer expects array segment",
				nil,
			)
		}

		next, err := setPointerValue(items[index], tail, value)
		if err != nil {
			return nil, err
		}
		items[index] = next
		return items, nil
	}

	var fields map[string]any
	switch typed := root.(type) {
	case nil:
		fields = map[string]any{}
	case map[string]any:
		fields = typed
	default:
		return nil, faults.NewTypedError(
			faults.ValidationError,
			"compare pointer expects object segment",
			nil,
		)
	}

	next, err := setPointerValue(fields[head], tail, value)
	if err != nil {
		return nil, err
	}
	fields[head] = next
	return fields, nil
}

func deletePointerValue(root any, tokens []string) (any, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseArrayIndex(head); isIndex {
		items, ok := root.([]any)
		if !ok {
			return root, nil
		}
		if index < 0 || index >= len(items) {
			return root, nil
		}

		if len(tail) == 0 {
			return append(items[:index], items[index+1:]...), nil
		}

		next, err := deletePointerValue(items[index], tail)
		if err != nil {
			return nil, err
		}
		items[index] = next
		return items, nil
	}

	fields, ok := root.(map[string]any)
	if !ok {
		return root, nil
	}

	if len(tail) == 0 {
		delete(fields, head)
		return fields, nil
	}

	child, found := fields[head]
	if !found {
		return root, nil
	}

	next, err := deletePointerValue(child, tail)
	if err != nil {
		return nil, err
	}
	if next == nil {
		delete(fields, head)
		return fields, nil
	}

	fields[head] = next
	return fields, nil
}

func parseArrayIndex(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}

	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return index, true
}

func deepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = deepCopyValue(item)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for idx := range typed {
			copied[idx] = deepCopyValue(typed[idx])
		}
		return copied
	default:
		return typed
	}
}
