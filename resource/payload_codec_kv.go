// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/crmarques/declarest/faults"
)

func decodeINIPayload(data []byte) (map[string]any, error) {
	lines := splitPayloadLines(data)
	root := map[string]any{}
	current := root

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if section == "" {
				return nil, faults.Invalid("invalid ini payload", fmt.Errorf("empty section name"))
			}
			existing, found := root[section]
			if !found {
				sectionMap := map[string]any{}
				root[section] = sectionMap
				current = sectionMap
				continue
			}
			sectionMap, ok := existing.(map[string]any)
			if !ok {
				return nil, faults.Invalid(
					"invalid ini payload",
					fmt.Errorf("section %q conflicts with a scalar key", section),
				)
			}
			current = sectionMap
			continue
		}

		key, value, err := parseINILine(trimmed)
		if err != nil {
			return nil, faults.Invalid("invalid ini payload", err)
		}
		current[key] = value
	}

	return root, nil
}

func parseINILine(line string) (string, string, error) {
	for idx, r := range line {
		switch r {
		case '=', ':':
			key := strings.TrimSpace(line[:idx])
			if key == "" {
				return "", "", fmt.Errorf("missing key")
			}
			return key, strings.TrimSpace(line[idx+1:]), nil
		}
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", "", fmt.Errorf("missing key")
	}
	key := fields[0]
	if len(key) == len(strings.TrimSpace(line)) {
		return key, "", nil
	}
	return key, strings.TrimSpace(line[len(key):]), nil
}

func encodeINIPayload(value any) ([]byte, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, faults.Invalid("failed to encode ini payload", fmt.Errorf("ini payload requires an object"))
	}

	rootKeys := make([]string, 0, len(root))
	sectionKeys := make([]string, 0, len(root))
	for key, item := range root {
		if _, ok := item.(map[string]any); ok {
			sectionKeys = append(sectionKeys, key)
			continue
		}
		rootKeys = append(rootKeys, key)
	}
	slices.Sort(rootKeys)
	slices.Sort(sectionKeys)

	lines := make([]string, 0, len(rootKeys)+len(sectionKeys)*2)
	for _, key := range rootKeys {
		text, err := stringifyStructuredTextScalar(root[key], "ini")
		if err != nil {
			return nil, err
		}
		lines = append(lines, fmt.Sprintf("%s = %s", key, text))
	}

	for idx, section := range sectionKeys {
		sectionMap, _ := root[section].(map[string]any)
		if len(lines) > 0 || idx > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("[%s]", section))

		keys := slices.Sorted(maps.Keys(sectionMap))
		for _, key := range keys {
			text, err := stringifyStructuredTextScalar(sectionMap[key], "ini")
			if err != nil {
				return nil, err
			}
			lines = append(lines, fmt.Sprintf("%s = %s", key, text))
		}
	}

	if len(lines) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func decodePropertiesPayload(data []byte) (map[string]any, error) {
	lines := propertiesLogicalLines(data)
	values := make(map[string]any)

	for _, rawLine := range lines {
		trimmed := strings.TrimLeft(rawLine, " \t\f")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}

		keyRaw, valueRaw := splitPropertiesKeyValue(trimmed)
		key, err := unescapePropertiesString(keyRaw)
		if err != nil {
			return nil, faults.Invalid("invalid properties payload", err)
		}
		value, err := unescapePropertiesString(valueRaw)
		if err != nil {
			return nil, faults.Invalid("invalid properties payload", err)
		}
		values[key] = value
	}

	return values, nil
}

func propertiesLogicalLines(data []byte) []string {
	lines := splitPayloadLines(data)
	logical := make([]string, 0, len(lines))
	current := ""

	for idx, line := range lines {
		if idx == 0 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}

		if current == "" {
			current = line
		} else {
			current = current[:len(current)-1] + strings.TrimLeft(line, " \t\f")
		}

		if hasTrailingUnescapedBackslash(current) {
			continue
		}

		logical = append(logical, current)
		current = ""
	}

	if current != "" {
		logical = append(logical, current)
	}

	return logical
}

func splitPayloadLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func hasTrailingUnescapedBackslash(value string) bool {
	count := 0
	for idx := len(value) - 1; idx >= 0 && value[idx] == '\\'; idx-- {
		count++
	}
	return count%2 == 1
}

func splitPropertiesKeyValue(line string) (string, string) {
	separatorIdx := -1
	separatorRune := rune(0)
	escaped := false

	for idx, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '=' || r == ':' {
			separatorIdx = idx
			separatorRune = r
			break
		}
		if r == ' ' || r == '\t' || r == '\f' {
			separatorIdx = idx
			separatorRune = r
			break
		}
	}

	if separatorIdx < 0 {
		return line, ""
	}

	key := line[:separatorIdx]
	valueStart := separatorIdx
	if separatorRune == '=' || separatorRune == ':' {
		valueStart++
	}
	for valueStart < len(line) {
		switch line[valueStart] {
		case ' ', '\t', '\f':
			valueStart++
			continue
		case '=', ':':
			if separatorRune == ' ' || separatorRune == '\t' || separatorRune == '\f' {
				valueStart++
				continue
			}
		}
		break
	}

	return key, line[valueStart:]
}

func unescapePropertiesString(value string) (string, error) {
	var builder strings.Builder

	for idx := 0; idx < len(value); idx++ {
		ch := value[idx]
		if ch != '\\' {
			builder.WriteByte(ch)
			continue
		}

		idx++
		if idx >= len(value) {
			builder.WriteByte('\\')
			break
		}

		switch value[idx] {
		case 't':
			builder.WriteByte('\t')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 'f':
			builder.WriteByte('\f')
		case '\\', '=', ':', '#', '!', ' ':
			builder.WriteByte(value[idx])
		case 'u':
			if idx+4 >= len(value) {
				return "", fmt.Errorf("truncated unicode escape")
			}
			hexValue := value[idx+1 : idx+5]
			codePoint, err := strconv.ParseUint(hexValue, 16, 16)
			if err != nil {
				return "", fmt.Errorf("invalid unicode escape %q", "\\u"+hexValue)
			}
			builder.WriteRune(rune(codePoint))
			idx += 4
		default:
			builder.WriteByte(value[idx])
		}
	}

	return builder.String(), nil
}

func encodePropertiesPayload(value any) ([]byte, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return nil, faults.Invalid("failed to encode properties payload", fmt.Errorf("properties payload requires an object"))
	}

	keys := slices.Sorted(maps.Keys(root))
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		text, err := stringifyStructuredTextScalar(root[key], "properties")
		if err != nil {
			return nil, err
		}
		lines = append(lines, escapePropertiesKey(key)+"="+escapePropertiesValue(text))
	}

	if len(lines) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func escapePropertiesKey(value string) string {
	var builder strings.Builder
	for idx, r := range value {
		switch r {
		case '\\':
			builder.WriteString("\\\\")
		case '=', ':', '#', '!':
			builder.WriteByte('\\')
			builder.WriteRune(r)
		case ' ':
			if idx == 0 {
				builder.WriteString("\\ ")
				continue
			}
			builder.WriteRune(r)
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func escapePropertiesValue(value string) string {
	var builder strings.Builder
	for idx, r := range value {
		switch r {
		case '\\':
			builder.WriteString("\\\\")
		case '\t':
			builder.WriteString("\\t")
		case '\n':
			builder.WriteString("\\n")
		case '\r':
			builder.WriteString("\\r")
		case '\f':
			builder.WriteString("\\f")
		case ' ':
			if idx == 0 {
				builder.WriteString("\\ ")
				continue
			}
			builder.WriteRune(r)
		default:
			if r == utf8.RuneError {
				builder.WriteRune(r)
				continue
			}
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func stringifyStructuredTextScalar(value any, payloadType string) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case map[string]any:
		return "", faults.Invalid(
			fmt.Sprintf("failed to encode %s payload", payloadType),
			fmt.Errorf("%s payloads support only root objects and scalar values", payloadType),
		)
	case []any:
		return "", faults.Invalid(
			fmt.Sprintf("failed to encode %s payload", payloadType),
			fmt.Errorf("%s payloads do not support arrays", payloadType),
		)
	default:
		return fmt.Sprint(typed), nil
	}
}
