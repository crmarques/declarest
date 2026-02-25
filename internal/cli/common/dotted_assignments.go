package common

import "strings"

func ParseDottedAssignmentsObject(raw string) (map[string]any, error) {
	output := map[string]any{}
	if err := ApplyDottedAssignmentsObject(output, raw); err != nil {
		return nil, err
	}
	return output, nil
}

func ApplyDottedAssignmentsObject(target map[string]any, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ValidationError("invalid assignment list: expected key=value", nil)
	}

	for _, item := range strings.Split(trimmed, ",") {
		part := strings.TrimSpace(item)
		if part == "" {
			return ValidationError("invalid assignment list: empty item", nil)
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			return ValidationError("invalid assignment list: expected key=value", nil)
		}

		key := strings.TrimSpace(pieces[0])
		if key == "" {
			return ValidationError("invalid assignment list: key must not be empty", nil)
		}
		value := strings.TrimSpace(pieces[1])

		if err := setDottedAssignmentValue(target, key, value); err != nil {
			return err
		}
	}

	return nil
}

func setDottedAssignmentValue(target map[string]any, dottedKey string, value string) error {
	segments := strings.Split(strings.TrimSpace(dottedKey), ".")
	current := target
	for idx, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return ValidationError("invalid assignment key: empty path segment", nil)
		}
		isLeaf := idx == len(segments)-1
		if isLeaf {
			current[segment] = value
			return nil
		}

		next, exists := current[segment]
		if !exists {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}

		child, ok := next.(map[string]any)
		if !ok {
			return ValidationError("invalid assignment list: key path conflicts with scalar value", nil)
		}
		current = child
	}

	return nil
}
