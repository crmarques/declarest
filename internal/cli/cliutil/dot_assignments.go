package cliutil

import (
	"strings"
)

// ParseDotNotationAssignmentsObject parses a comma-separated list of
// dot-notation key=value assignments into a nested map.
//
// Syntax:
//
//	key=value                → {"key": "value"}
//	a.b=value                → {"a": {"b": "value"}}
//	a."b.c"=value            → {"a": {"b.c": "value"}}
//	a.b=x,c.d=y             → {"a": {"b": "x"}, "c": {"d": "y"}}
//
// Quoted segments (double quotes) preserve literal dots inside the key segment.
func ParseDotNotationAssignmentsObject(raw string) (map[string]any, error) {
	output := map[string]any{}
	if err := ApplyDotNotationAssignmentsObject(output, raw); err != nil {
		return nil, err
	}
	return output, nil
}

// ApplyDotNotationAssignmentsObject applies dot-notation assignments to an
// existing target map.
func ApplyDotNotationAssignmentsObject(target map[string]any, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ValidationError("invalid assignment list: expected <key>=<value>", nil)
	}

	items, err := splitDotAssignmentItems(trimmed)
	if err != nil {
		return err
	}

	for _, item := range items {
		part := strings.TrimSpace(item)
		if part == "" {
			return ValidationError("invalid assignment list: empty item", nil)
		}

		eqIdx := strings.IndexByte(part, '=')
		if eqIdx < 0 {
			return ValidationError("invalid assignment list: expected <key>=<value>", nil)
		}

		keyRaw := strings.TrimSpace(part[:eqIdx])
		value := strings.TrimSpace(part[eqIdx+1:])

		if keyRaw == "" {
			return ValidationError("invalid assignment list: key must not be empty", nil)
		}

		segments, err := parseDotNotationKey(keyRaw)
		if err != nil {
			return err
		}
		if len(segments) == 0 {
			return ValidationError("invalid assignment list: key must not be empty", nil)
		}

		if err := setNestedValue(target, segments, value); err != nil {
			return err
		}
	}

	return nil
}

// IsDotNotationAssignment returns true when the input looks like one or more
// dot-notation key=value assignments (e.g. "name=test" or "a.b=x,c=y") rather
// than a JSON pointer list, a file path, or an inline JSON/YAML literal.
func IsDotNotationAssignment(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	// Must not start with '/' (JSON pointer) or '{' / '[' (inline literal).
	if trimmed[0] == '/' || trimmed[0] == '{' || trimmed[0] == '[' {
		return false
	}
	// Must contain at least one unquoted '=' sign.
	inQuote := false
	for _, ch := range trimmed {
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == '=' && !inQuote:
			return true
		}
	}
	return false
}

// splitDotAssignmentItems splits on commas that are not inside double-quoted
// segments.
func splitDotAssignmentItems(raw string) ([]string, error) {
	var items []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteByte(ch)
		case ch == ',' && !inQuote:
			items = append(items, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	if inQuote {
		return nil, ValidationError("invalid assignment list: unclosed quote in key", nil)
	}
	items = append(items, current.String())
	return items, nil
}

// parseDotNotationKey splits a dot-notation key into its constituent segments.
//
// Examples:
//
//	"name"              → ["name"]
//	"metadata.name"     → ["metadata", "name"]
//	`testA."testB.testC"` → ["testA", "testB.testC"]
func parseDotNotationKey(key string) ([]string, error) {
	var segments []string
	var current strings.Builder
	i := 0

	for i < len(key) {
		ch := key[i]

		switch ch {
		case '"':
			// Quoted segment: collect until closing quote.
			i++ // skip opening quote
			for i < len(key) && key[i] != '"' {
				current.WriteByte(key[i])
				i++
			}
			if i >= len(key) {
				return nil, ValidationError("invalid assignment list: unclosed quote in key", nil)
			}
			i++ // skip closing quote
		case '.':
			segment := current.String()
			if segment == "" {
				return nil, ValidationError("invalid assignment list: empty key segment", nil)
			}
			segments = append(segments, segment)
			current.Reset()
			i++
		default:
			current.WriteByte(ch)
			i++
		}
	}

	last := current.String()
	if last == "" && len(segments) > 0 {
		return nil, ValidationError("invalid assignment list: trailing dot in key", nil)
	}
	if last != "" {
		segments = append(segments, last)
	}

	return segments, nil
}

// setNestedValue sets a value in a nested map structure given a list of key
// segments.
func setNestedValue(target map[string]any, segments []string, value any) error {
	current := target
	for idx := 0; idx < len(segments)-1; idx++ {
		key := segments[idx]
		existing, ok := current[key]
		if !ok {
			child := map[string]any{}
			current[key] = child
			current = child
			continue
		}
		child, ok := existing.(map[string]any)
		if !ok {
			return ValidationError("invalid assignment list: key conflicts with scalar value", nil)
		}
		current = child
	}

	current[segments[len(segments)-1]] = value
	return nil
}
