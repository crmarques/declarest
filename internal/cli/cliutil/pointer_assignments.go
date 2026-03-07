package cliutil

import (
	"strings"

	"github.com/crmarques/declarest/resource"
)

func ParsePointerAssignmentsObject(raw string) (map[string]any, error) {
	output := map[string]any{}
	if err := ApplyPointerAssignmentsObject(output, raw); err != nil {
		return nil, err
	}
	return output, nil
}

func ApplyPointerAssignmentsObject(target map[string]any, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ValidationError("invalid assignment list: expected <json-pointer>=<value>", nil)
	}

	for _, item := range strings.Split(trimmed, ",") {
		part := strings.TrimSpace(item)
		if part == "" {
			return ValidationError("invalid assignment list: empty item", nil)
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			return ValidationError("invalid assignment list: expected <json-pointer>=<value>", nil)
		}

		pointer := strings.TrimSpace(pieces[0])
		if pointer == "" {
			return ValidationError("invalid assignment list: JSON pointer must not be empty", nil)
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return ValidationError("invalid assignment list: JSON pointer is invalid", err)
		}

		value := strings.TrimSpace(pieces[1])
		if _, err := resource.SetJSONPointerValue(target, pointer, value); err != nil {
			return ValidationError("invalid assignment list: JSON pointer conflicts with scalar value", err)
		}
	}

	return nil
}
