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
	"strconv"
	"strings"
	"unicode"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func ResolvePayload(value resource.Value, getFn func(key string) (string, error)) (resource.Value, error) {
	return resolvePayloadWithResourceScope(value, "", getFn)
}

func ResolvePayloadForResource(
	value resource.Value,
	logicalPath string,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	return resolvePayloadWithResourceScope(value, logicalPath, getFn)
}

// ResolvePayloadDirectivesForResource resolves supported exact-placeholder
// directives in resource payloads. It always resolves payload descriptor
// placeholders and resolves {{secret ...}} only when getFn is provided.
func ResolvePayloadDirectivesForResource(
	value resource.Value,
	logicalPath string,
	descriptor resource.PayloadDescriptor,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	resolvedDescriptor := resource.NormalizePayloadDescriptor(descriptor)

	withDescriptor, err := resolvePayloadDescriptorDirectivesValue(normalized, resolvedDescriptor, 0)
	if err != nil {
		return nil, err
	}

	if getFn == nil {
		return withDescriptor, nil
	}

	if resolvedWholeResource, handled, err := ResolveWholeResourcePlaceholderForResource(
		withDescriptor,
		logicalPath,
		resolvedDescriptor,
		getFn,
	); handled || err != nil {
		return resolvedWholeResource, err
	}

	cache := make(map[string]string)
	output, err := resolvePayloadValue(withDescriptor, "", strings.TrimSpace(logicalPath), cache, getFn, 0)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func resolvePayloadWithResourceScope(
	value resource.Value,
	logicalPath string,
	getFn func(key string) (string, error),
) (resource.Value, error) {
	if getFn == nil {
		return nil, faults.Invalid("secret get function must not be nil", nil)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	cache := make(map[string]string)
	output, err := resolvePayloadValue(normalized, "", strings.TrimSpace(logicalPath), cache, getFn, 0)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func resolvePayloadDescriptorDirectivesValue(
	value any,
	descriptor resource.PayloadDescriptor,
	depth int,
) (any, error) {
	if depth > maxPayloadDepth {
		return nil, faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			child, err := resolvePayloadDescriptorDirectivesValue(typed[key], descriptor, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := resolvePayloadDescriptorDirectivesValue(typed[idx], descriptor, depth+1)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	case string:
		resolvedValue, isDirective, err := resolvePayloadDescriptorPlaceholder(typed, descriptor)
		if err != nil {
			return nil, err
		}
		if isDirective {
			return resolvedValue, nil
		}
		return typed, nil
	default:
		return typed, nil
	}
}

func resolvePayloadValue(
	value any,
	currentPath string,
	resourcePath string,
	cache map[string]string,
	getFn func(key string) (string, error),
	depth int,
) (any, error) {
	if depth > maxPayloadDepth {
		return nil, faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			child, err := resolvePayloadValue(typed[key], attributePath, resourcePath, cache, getFn, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := resolvePayloadValue(
				typed[idx],
				joinAttributePath(currentPath, strconv.Itoa(idx)),
				resourcePath,
				cache,
				getFn,
				depth+1,
			)
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

		resolvedKey, err := resolvePlaceholderStoreKey(key, isCurrent, currentPath, resourcePath)
		if err != nil {
			return nil, err
		}

		if cached, found := cache[resolvedKey]; found {
			return cached, nil
		}

		resolved, err := getFn(resolvedKey)
		if err != nil {
			return nil, err
		}
		cache[resolvedKey] = resolved

		return resolved, nil
	default:
		return typed, nil
	}
}

func resolvePayloadDescriptorPlaceholder(
	value string,
	descriptor resource.PayloadDescriptor,
) (resolved string, isPlaceholder bool, err error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false, nil
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}"))
	var name string
	switch {
	case strings.HasPrefix(inner, "payload_type"):
		name = "payload_type"
	case strings.HasPrefix(inner, "payload_media_type"):
		name = "payload_media_type"
	case strings.HasPrefix(inner, "payload_extension"):
		name = "payload_extension"
	default:
		return "", false, nil
	}
	if len(inner) > len(name) {
		next := rune(inner[len(name)])
		if !unicode.IsSpace(next) {
			return "", false, nil
		}
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, name))
	if argument == "" {
		return "", true, faults.Invalid(name+" placeholder argument is required", nil)
	}
	if argument != "." {
		return "", true, faults.Invalid(name+" placeholder supports only {{"+name+" .}}", nil)
	}

	switch name {
	case "payload_type":
		return descriptor.PayloadType, true, nil
	case "payload_media_type":
		return descriptor.MediaType, true, nil
	case "payload_extension":
		return descriptor.Extension, true, nil
	default:
		return "", false, nil
	}
}
