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
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
)

func ParseJSONPointer(pointer string) ([]string, error) {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		return nil, faults.NewValidationError(fmt.Sprintf("invalid JSON pointer %q", pointer), nil)
	}

	rawTokens := strings.Split(trimmed[1:], "/")
	tokens := make([]string, len(rawTokens))
	for idx, token := range rawTokens {
		unescaped, err := unescapeJSONPointerToken(token)
		if err != nil {
			return nil, faults.NewValidationError(fmt.Sprintf("invalid JSON pointer %q", pointer), err)
		}
		tokens[idx] = unescaped
	}
	return tokens, nil
}

func EscapeJSONPointerToken(token string) string {
	escaped := strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(escaped, "/", "~1")
}

func JSONPointerFromTokens(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, token := range tokens {
		builder.WriteByte('/')
		builder.WriteString(EscapeJSONPointerToken(token))
	}
	return builder.String()
}

func JSONPointerForObjectKey(key string) string {
	return JSONPointerFromTokens([]string{key})
}

func LookupJSONPointer(value any, pointer string) (any, bool, error) {
	tokens, err := ParseJSONPointer(pointer)
	if err != nil {
		return nil, false, err
	}

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
			index, ok := parseJSONPointerArrayIndex(token)
			if !ok || index < 0 || index >= len(typed) {
				return nil, false, nil
			}
			current = typed[index]
		default:
			return nil, false, nil
		}
	}

	return current, true, nil
}

func LookupJSONPointerString(value any, pointer string) (string, bool, error) {
	item, found, err := LookupJSONPointer(value, pointer)
	if err != nil || !found {
		return "", found, err
	}
	text, ok := jsonPointerScalarString(item)
	return text, ok, nil
}

func SetJSONPointerValue(root any, pointer string, value any) (any, error) {
	tokens, err := ParseJSONPointer(pointer)
	if err != nil {
		return nil, err
	}
	return setJSONPointerValue(root, tokens, value)
}

func DeleteJSONPointerValue(root any, pointer string) (any, error) {
	tokens, err := ParseJSONPointer(pointer)
	if err != nil {
		return nil, err
	}
	return deleteJSONPointerValue(root, tokens)
}

func DeepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = DeepCopyValue(item)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for idx := range typed {
			copied[idx] = DeepCopyValue(typed[idx])
		}
		return copied
	default:
		return typed
	}
}

func unescapeJSONPointerToken(token string) (string, error) {
	if !strings.ContainsRune(token, '~') {
		return token, nil
	}

	var builder strings.Builder
	builder.Grow(len(token))
	for idx := 0; idx < len(token); idx++ {
		if token[idx] != '~' {
			builder.WriteByte(token[idx])
			continue
		}
		if idx+1 >= len(token) {
			return "", faults.NewValidationError("invalid JSON pointer escape", nil)
		}
		switch token[idx+1] {
		case '0':
			builder.WriteByte('~')
		case '1':
			builder.WriteByte('/')
		default:
			return "", faults.NewValidationError("invalid JSON pointer escape", nil)
		}
		idx++
	}
	return builder.String(), nil
}

func parseJSONPointerArrayIndex(value string) (int, bool) {
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

func setJSONPointerValue(root any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return DeepCopyValue(value), nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseJSONPointerArrayIndex(head); isIndex {
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
			return nil, faults.NewValidationError("JSON pointer expects array segment", nil)
		}

		next, err := setJSONPointerValue(items[index], tail, value)
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
		return nil, faults.NewValidationError("JSON pointer expects object segment", nil)
	}

	next, err := setJSONPointerValue(fields[head], tail, value)
	if err != nil {
		return nil, err
	}
	fields[head] = next
	return fields, nil
}

func deleteJSONPointerValue(root any, tokens []string) (any, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseJSONPointerArrayIndex(head); isIndex {
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

		next, err := deleteJSONPointerValue(items[index], tail)
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

	next, err := deleteJSONPointerValue(child, tail)
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

func jsonPointerScalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, typed != ""
	case fmt.Stringer:
		text := strings.TrimSpace(typed.String())
		return text, text != ""
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}
