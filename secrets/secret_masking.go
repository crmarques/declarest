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

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func MaskPayload(value resource.Value, storeFn func(key string, value string) error) (resource.Value, error) {
	if storeFn == nil {
		return nil, faults.Invalid("secret store function must not be nil", nil)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]string)
	scopeByKey := make(map[string]string)
	if err := collectMaskCandidates(normalized, "", candidates, scopeByKey, 0); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := storeFn(key, candidates[key]); err != nil {
			return nil, err
		}
	}

	output, err := applyMask(normalized, "", candidates, 0)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func collectMaskCandidates(
	value any,
	currentPath string,
	candidates map[string]string,
	scopeByKey map[string]string,
	depth int,
) error {
	if depth > maxPayloadDepth {
		return faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			field := typed[key]
			if isLikelySecretKey(key) {
				stringValue, isString := field.(string)
				if !isString {
					if field != nil {
						return faults.Invalid("secret masking supports only string values for detected keys", nil)
					}
				} else {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return err
					}
					if !isPlaceholder {
						if existingPath, found := scopeByKey[key]; found && existingPath != attributePath {
							return faults.Invalid("secret masking key scope is ambiguous", nil)
						}
						scopeByKey[key] = attributePath

						if _, found := candidates[attributePath]; found {
							return faults.Invalid("secret masking key scope is ambiguous", nil)
						}
						candidates[attributePath] = stringValue
					}
				}
			}

			if err := collectMaskCandidates(field, attributePath, candidates, scopeByKey, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for idx := range typed {
			if err := collectMaskCandidates(
				typed[idx],
				joinAttributePath(currentPath, strconv.Itoa(idx)),
				candidates,
				scopeByKey,
				depth+1,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyMask(value any, currentPath string, candidates map[string]string, depth int) (any, error) {
	if depth > maxPayloadDepth {
		return nil, faults.Invalid("secret payload exceeds maximum nesting depth", nil)
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for _, key := range sortedKeys(typed) {
			attributePath := joinAttributePath(currentPath, key)
			field := typed[key]
			if _, shouldMask := candidates[attributePath]; shouldMask {
				stringValue, isString := field.(string)
				if isString {
					_, _, isPlaceholder, err := parseSecretPlaceholder(stringValue)
					if err != nil {
						return nil, err
					}
					if !isPlaceholder {
						result[key] = currentScopeSecretPlaceholder()
						continue
					}
				}
			}

			child, err := applyMask(field, attributePath, candidates, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			child, err := applyMask(typed[idx], joinAttributePath(currentPath, strconv.Itoa(idx)), candidates, depth+1)
			if err != nil {
				return nil, err
			}
			result[idx] = child
		}
		return result, nil
	default:
		return typed, nil
	}
}
