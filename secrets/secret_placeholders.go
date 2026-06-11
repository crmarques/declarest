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

package secrets

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

const maxPayloadDepth = 256

func NormalizePlaceholders(value resource.Value) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	output, err := normalizePlaceholdersValue(normalized, "", 0)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func normalizePlaceholdersValue(value any, currentPath string, depth int) (any, error) {
	if depth > maxPayloadDepth {
		return nil, faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			child, err := normalizePlaceholdersValue(typed[key], attributePath, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := normalizePlaceholdersValue(typed[idx], joinAttributePath(currentPath, strconv.Itoa(idx)), depth+1)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	case string:
		key, isCurrent, isPlaceholder, err := parseSecretPlaceholder(typed)
		if err != nil {
			return nil, err
		}
		if !isPlaceholder {
			return typed, nil
		}
		resolvedKey, err := resolvePlaceholderAttribute(key, isCurrent, currentPath)
		if err != nil {
			return nil, err
		}
		if resolvedKey == currentPath {
			return currentScopeSecretPlaceholder(), nil
		}
		return explicitSecretPlaceholder(resolvedKey), nil
	default:
		return typed, nil
	}
}

func parseSecretPlaceholder(value string) (key string, isCurrent bool, isPlaceholder bool, err error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false, false, nil
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}")
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "secret") {
		return "", false, false, nil
	}
	if len(inner) > len("secret") {
		next := rune(inner[len("secret")])
		if !unicode.IsSpace(next) {
			return "", false, false, nil
		}
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "secret"))
	if argument == "" {
		return "", false, true, faults.Invalid("secret placeholder argument is required", nil)
	}

	if argument == "." {
		return "", true, true, nil
	}

	if strings.HasPrefix(argument, "\"") {
		parsed, parseErr := strconv.Unquote(argument)
		if parseErr != nil {
			return "", false, true, faults.Invalid("secret placeholder key is invalid", parseErr)
		}

		parsed = strings.TrimSpace(parsed)
		if parsed == "" {
			return "", false, true, faults.Invalid("secret placeholder key must not be empty", nil)
		}
		return parsed, false, true, nil
	}

	if strings.ContainsAny(argument, " \t\r\n") {
		return "", false, true, faults.Invalid("secret placeholder key with spaces must be quoted", nil)
	}

	return argument, false, true, nil
}

func resolvePlaceholderStoreKey(
	key string,
	isCurrent bool,
	currentPath string,
	resourcePath string,
) (string, error) {
	resolvedAttribute, err := resolvePlaceholderAttribute(key, isCurrent, currentPath)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(resourcePath) == "" {
		return resolvedAttribute, nil
	}

	return strings.TrimSpace(resourcePath) + ":" + resolvedAttribute, nil
}

func resolvePlaceholderAttribute(key string, isCurrent bool, currentPath string) (string, error) {
	if !isCurrent {
		resolved := strings.TrimSpace(key)
		if resolved == "" {
			return "", faults.Invalid("secret placeholder key must not be empty", nil)
		}
		if strings.HasPrefix(resolved, "/") {
			return "", faults.Invalid("secret placeholder key must be relative to the resource path", nil)
		}
		return resolved, nil
	}

	resolved := strings.TrimSpace(currentPath)
	if resolved == "" {
		return "", faults.Invalid("secret placeholder {{secret .}} requires map field scope", nil)
	}

	return resolved, nil
}

func currentScopeSecretPlaceholder() string {
	return "{{secret .}}"
}

func explicitSecretPlaceholder(key string) string {
	return "{{secret " + strconv.Quote(key) + "}}"
}

func joinAttributePath(prefix string, key string) string {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return ""
	}
	escapedKey := resource.EscapeJSONPointerToken(trimmedKey)
	if strings.TrimSpace(prefix) == "" {
		return "/" + escapedKey
	}
	return prefix + "/" + escapedKey
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
