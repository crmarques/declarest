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
	"reflect"
	"sort"

	"github.com/crmarques/declarest/faults"
)

var defaultsOverlayPayloadTypes = []string{
	PayloadTypeINI,
	PayloadTypeJSON,
	PayloadTypeProperties,
	PayloadTypeYAML,
}

func SupportedDefaultsOverlayPayloadTypes() []string {
	values := make([]string, len(defaultsOverlayPayloadTypes))
	copy(values, defaultsOverlayPayloadTypes)
	sort.Strings(values)
	return values
}

func SupportsDefaultsOverlayPayloadType(payloadType string) bool {
	normalized := NormalizePayloadType(payloadType)
	for _, candidate := range defaultsOverlayPayloadTypes {
		if normalized == candidate {
			return true
		}
	}
	return false
}

func MergeWithDefaults(defaults Value, overrides Value) (Value, error) {
	if defaults == nil {
		return Normalize(overrides)
	}

	normalizedDefaults, err := Normalize(defaults)
	if err != nil {
		return nil, err
	}
	defaultObject, ok := normalizedDefaults.(map[string]any)
	if !ok {
		return nil, faults.NewValidationError("resource defaults require a structured object payload", nil)
	}

	if overrides == nil {
		return Normalize(DeepCopyValue(defaultObject))
	}

	normalizedOverrides, err := Normalize(overrides)
	if err != nil {
		return nil, err
	}
	overrideObject, ok := normalizedOverrides.(map[string]any)
	if !ok {
		return nil, faults.NewValidationError("resource payload overrides must be a structured object when resource defaults are present", nil)
	}

	return Normalize(mergeObjectWithDefaults(defaultObject, overrideObject))
}

func CompactAgainstDefaults(value Value, defaults Value) (Value, error) {
	if defaults == nil {
		return Normalize(value)
	}

	normalizedDefaults, err := Normalize(defaults)
	if err != nil {
		return nil, err
	}
	defaultObject, ok := normalizedDefaults.(map[string]any)
	if !ok {
		return nil, faults.NewValidationError("resource defaults require a structured object payload", nil)
	}

	if value == nil {
		return nil, nil
	}

	normalizedValue, err := Normalize(value)
	if err != nil {
		return nil, err
	}
	valueObject, ok := normalizedValue.(map[string]any)
	if !ok {
		return nil, faults.NewValidationError("effective payload must be a structured object when resource defaults are present", nil)
	}

	compacted := compactObjectAgainstDefaults(valueObject, defaultObject)
	if len(compacted) == 0 {
		return nil, nil
	}
	return Normalize(compacted)
}

func ValidateDefaultsSidecarDescriptor(defaults PayloadDescriptor, overrides PayloadDescriptor) error {
	resolvedDefaults := NormalizePayloadDescriptor(defaults)
	if !SupportsDefaultsOverlayPayloadType(resolvedDefaults.PayloadType) {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resource defaults require merge-capable payload type (json, yaml, ini, properties); got %q",
				resolvedDefaults.PayloadType,
			),
			nil,
		)
	}

	if !IsPayloadDescriptorExplicit(overrides) {
		return nil
	}

	resolvedOverrides := NormalizePayloadDescriptor(overrides)
	if !defaultsOverlayPayloadTypesCompatible(resolvedDefaults.PayloadType, resolvedOverrides.PayloadType) {
		return faults.NewValidationError(
			fmt.Sprintf(
				"resource defaults payload type %q does not match resource payload type %q",
				resolvedDefaults.PayloadType,
				resolvedOverrides.PayloadType,
			),
			nil,
		)
	}

	return nil
}

func defaultsOverlayPayloadTypesCompatible(defaultsType string, overridesType string) bool {
	normalizedDefaults := NormalizePayloadType(defaultsType)
	normalizedOverrides := NormalizePayloadType(overridesType)
	if normalizedDefaults == normalizedOverrides {
		return true
	}
	return isJSONYAMLDefaultsOverlayType(normalizedDefaults) && isJSONYAMLDefaultsOverlayType(normalizedOverrides)
}

func isJSONYAMLDefaultsOverlayType(payloadType string) bool {
	switch NormalizePayloadType(payloadType) {
	case PayloadTypeJSON, PayloadTypeYAML:
		return true
	default:
		return false
	}
}

func InferDefaultsFromValues(values ...Value) (Value, error) {
	if len(values) < 2 {
		return Normalize(map[string]any{})
	}

	var common map[string]any
	samples := 0
	for _, value := range values {
		if value == nil {
			continue
		}

		normalized, err := Normalize(value)
		if err != nil {
			return nil, err
		}
		current, ok := normalized.(map[string]any)
		if !ok {
			return nil, faults.NewValidationError("defaults inference requires structured object payloads", nil)
		}

		if samples == 0 {
			common = DeepCopyValue(current).(map[string]any)
			samples++
			continue
		}

		common = intersectObjectDefaults(common, current)
		samples++
	}

	if samples < 2 || len(common) == 0 {
		return Normalize(map[string]any{})
	}

	return Normalize(common)
}

func InferCreatedDefaults(inputs []Value, outputs []Value) (Value, error) {
	inferredOutputs, err := InferDefaultsFromValues(outputs...)
	if err != nil {
		return nil, err
	}

	inferredInputs, err := InferDefaultsFromValues(inputs...)
	if err != nil {
		return nil, err
	}

	compacted, err := CompactAgainstDefaults(inferredOutputs, inferredInputs)
	if err != nil {
		return nil, err
	}
	if compacted == nil {
		return Normalize(map[string]any{})
	}
	return Normalize(compacted)
}

func ValidateDefaultsSidecarValue(defaults Value) error {
	if defaults == nil {
		return faults.NewValidationError("resource defaults require a structured object payload", nil)
	}

	normalizedDefaults, err := Normalize(defaults)
	if err != nil {
		return err
	}
	if _, ok := normalizedDefaults.(map[string]any); !ok {
		return faults.NewValidationError("resource defaults require a structured object payload", nil)
	}
	return nil
}

func mergeObjectWithDefaults(defaults map[string]any, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = DeepCopyValue(value)
	}
	for key, overrideValue := range overrides {
		defaultValue, hasDefault := defaults[key]
		if hasDefault {
			defaultObject, defaultIsObject := defaultValue.(map[string]any)
			overrideObject, overrideIsObject := overrideValue.(map[string]any)
			if defaultIsObject && overrideIsObject {
				merged[key] = mergeObjectWithDefaults(defaultObject, overrideObject)
				continue
			}
		}
		merged[key] = DeepCopyValue(overrideValue)
	}
	return merged
}

func compactObjectAgainstDefaults(value map[string]any, defaults map[string]any) map[string]any {
	compacted := make(map[string]any)
	for key, currentValue := range value {
		defaultValue, hasDefault := defaults[key]
		if !hasDefault {
			compacted[key] = DeepCopyValue(currentValue)
			continue
		}

		defaultObject, defaultIsObject := defaultValue.(map[string]any)
		valueObject, valueIsObject := currentValue.(map[string]any)
		if defaultIsObject && valueIsObject {
			nested := compactObjectAgainstDefaults(valueObject, defaultObject)
			if len(nested) > 0 {
				compacted[key] = nested
			}
			continue
		}

		if reflect.DeepEqual(currentValue, defaultValue) {
			continue
		}
		compacted[key] = DeepCopyValue(currentValue)
	}
	return compacted
}

func intersectObjectDefaults(left map[string]any, right map[string]any) map[string]any {
	result := make(map[string]any)
	for key, leftValue := range left {
		rightValue, found := right[key]
		if !found {
			continue
		}

		leftObject, leftIsObject := leftValue.(map[string]any)
		rightObject, rightIsObject := rightValue.(map[string]any)
		if leftIsObject && rightIsObject {
			nested := intersectObjectDefaults(leftObject, rightObject)
			if len(nested) > 0 || (len(leftObject) == 0 && len(rightObject) == 0) {
				result[key] = nested
			}
			continue
		}

		if reflect.DeepEqual(leftValue, rightValue) {
			result[key] = DeepCopyValue(leftValue)
		}
	}
	return result
}
