package metadata

import (
	"strconv"
	"strings"

	"github.com/crmarques/declarest/resource"
)

var reservedTemplateIdentifiers = map[string]struct{}{
	"and":                {},
	"block":              {},
	"call":               {},
	"define":             {},
	"else":               {},
	"end":                {},
	"eq":                 {},
	"false":              {},
	"ge":                 {},
	"gt":                 {},
	"html":               {},
	"if":                 {},
	"index":              {},
	"js":                 {},
	"json_pointer":       {},
	"le":                 {},
	"len":                {},
	"lt":                 {},
	"ne":                 {},
	"nil":                {},
	"not":                {},
	"or":                 {},
	"payload_extension":  {},
	"payload_media_type": {},
	"payload_type":       {},
	"print":              {},
	"printf":             {},
	"println":            {},
	"range":              {},
	"slice":              {},
	"template":           {},
	"true":               {},
	"urlquery":           {},
	"with":               {},
}

func rewriteMetadataTemplateSyntax(raw string) string {
	if !strings.Contains(raw, "{{") {
		return raw
	}

	var builder strings.Builder
	offset := 0
	for {
		start := strings.Index(raw[offset:], "{{")
		if start < 0 {
			builder.WriteString(raw[offset:])
			break
		}
		start += offset
		builder.WriteString(raw[offset:start])

		end := strings.Index(raw[start+2:], "}}")
		if end < 0 {
			builder.WriteString(raw[start:])
			break
		}
		end += start + 2

		action := raw[start : end+2]
		if pointer, ok := templateExpressionPointer(strings.TrimSpace(raw[start+2 : end])); ok {
			builder.WriteString("{{json_pointer ")
			builder.WriteString(strconv.Quote(pointer))
			builder.WriteString("}}")
		} else {
			builder.WriteString(action)
		}

		offset = end + 2
	}

	return builder.String()
}

func templateExpressionPointer(expression string) (string, bool) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return "", false
	}

	if strings.HasPrefix(trimmed, ".") {
		key := strings.TrimSpace(strings.TrimPrefix(trimmed, "."))
		if templateIdentifierPattern.MatchString(key) {
			return resource.JSONPointerForObjectKey(key), true
		}
		return "", false
	}

	if strings.HasPrefix(trimmed, "/") {
		if _, err := resource.ParseJSONPointer(trimmed); err == nil {
			return trimmed, true
		}
		return "", false
	}

	if !templateIdentifierPattern.MatchString(trimmed) {
		return "", false
	}
	if _, reserved := reservedTemplateIdentifiers[trimmed]; reserved {
		return "", false
	}

	return resource.JSONPointerForObjectKey(trimmed), true
}

func templatePlaceholderPointer(segment string) (string, bool) {
	trimmed := strings.TrimSpace(segment)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	return templateExpressionPointer(strings.TrimSpace(trimmed[2 : len(trimmed)-2]))
}

func TemplatePlaceholderKey(segment string) (string, bool) {
	pointer, ok := templatePlaceholderPointer(segment)
	if !ok {
		return "", false
	}

	tokens, err := resource.ParseJSONPointer(pointer)
	if err != nil || len(tokens) != 1 {
		return "", false
	}

	return tokens[0], true
}

func IsTemplatePlaceholderSegment(segment string) bool {
	_, ok := templatePlaceholderPointer(segment)
	return ok
}
