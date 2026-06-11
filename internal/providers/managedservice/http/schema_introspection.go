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

package http

import (
	"fmt"
	"math"
	"strings"
)

func topLevelSchemaObjectFieldNames(
	schema any,
	document map[string]any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > maxSchemaDepth {
		return nil
	}

	resolvedSchema, err := resolveSchemaValue(document, schema, visitedRefs, depth)
	if err != nil || len(resolvedSchema) == 0 {
		return nil
	}

	fields := map[string]struct{}{}
	for _, name := range requiredPropertyNames(resolvedSchema["required"]) {
		fields[name] = struct{}{}
	}
	if properties, ok := asStringAnyMap(resolvedSchema["properties"]); ok {
		for name := range properties {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			fields[trimmed] = struct{}{}
		}
	}

	if allOf, ok := schemaSlice(resolvedSchema["allOf"]); ok {
		for _, item := range allOf {
			merged := topLevelSchemaObjectFieldNames(item, document, map[string]struct{}{}, depth+1)
			for key := range merged {
				fields[key] = struct{}{}
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func requiredPropertyNames(value any) []string {
	rawValues, ok := schemaSlice(value)
	if !ok || len(rawValues) == 0 {
		return nil
	}

	names := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, raw := range rawValues {
		name, ok := raw.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		names = append(names, trimmed)
	}
	return names
}

func schemaTypeNames(value any) []string {
	if value == nil {
		return nil
	}
	if single, ok := value.(string); ok {
		trimmed := strings.ToLower(strings.TrimSpace(single))
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	rawValues, ok := schemaSlice(value)
	if !ok {
		return nil
	}

	types := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, raw := range rawValues {
		name, ok := raw.(string)
		if !ok {
			continue
		}
		trimmed := strings.ToLower(strings.TrimSpace(name))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		types = append(types, trimmed)
	}
	return types
}

func schemaTypeContains(value any, expected string) bool {
	expectedNormalized := strings.ToLower(strings.TrimSpace(expected))
	if expectedNormalized == "" {
		return false
	}
	for _, actual := range schemaTypeNames(value) {
		if actual == expectedNormalized {
			return true
		}
	}
	return false
}

func valueMatchesSchemaType(value any, schemaType string) bool {
	switch strings.ToLower(strings.TrimSpace(schemaType)) {
	case "null":
		return value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "integer":
		switch typed := value.(type) {
		case int:
			return true
		case int8:
			return true
		case int16:
			return true
		case int32:
			return true
		case int64:
			return true
		case uint:
			return true
		case uint8:
			return true
		case uint16:
			return true
		case uint32:
			return true
		case uint64:
			return true
		case float64:
			return math.Trunc(typed) == typed
		case float32:
			return math.Trunc(float64(typed)) == float64(typed)
		default:
			return false
		}
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func describeValueType(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func schemaSlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	default:
		return nil, false
	}
}

func asStringAnyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		mapped := make(map[string]any, len(typed))
		for key, item := range typed {
			keyText, ok := key.(string)
			if !ok {
				return nil, false
			}
			mapped[keyText] = item
		}
		return mapped, true
	default:
		return nil, false
	}
}

func dotPath(base string, field string) string {
	trimmedField := strings.TrimSpace(field)
	if trimmedField == "" {
		return base
	}
	if base == "" || base == "$" {
		return "$." + trimmedField
	}
	return base + "." + trimmedField
}
