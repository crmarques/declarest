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
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) validateOperationSchemaRef(
	ctx context.Context,
	payload resource.Value,
	derivedFields map[string]any,
	requestPath string,
	requestMethod string,
	schemaRef string,
) error {
	trimmedSchemaRef := strings.TrimSpace(schemaRef)
	if trimmedSchemaRef == "" {
		return nil
	}

	schema, document, err := g.resolveOpenAPISchemaForValidation(ctx, requestPath, requestMethod, trimmedSchemaRef)
	if err != nil {
		return err
	}

	schemaPayload := payload
	if len(derivedFields) > 0 {
		schemaPayload = augmentSchemaValidationPayload(payload, derivedFields, schema, document)
	}

	if err := validateValueAgainstOpenAPISchema(schemaPayload, schema, document, "$", map[string]struct{}{}, nil, 0); err != nil {
		return faults.Invalid(
			fmt.Sprintf(
				"operation payload validation failed for schemaRef %q: %v",
				trimmedSchemaRef,
				err,
			),
			nil,
		)
	}
	return nil
}

func augmentSchemaValidationPayload(
	payload resource.Value,
	derivedFields map[string]any,
	schema any,
	document map[string]any,
) resource.Value {
	payloadObject, ok := payload.(map[string]any)
	if !ok {
		return payload
	}

	allowedFields := topLevelSchemaObjectFieldNames(schema, document, map[string]struct{}{}, 0)
	if len(allowedFields) == 0 {
		return payload
	}

	allowedPointers := make(map[string]struct{}, len(allowedFields))
	for key := range allowedFields {
		allowedPointers[resource.JSONPointerForObjectKey(key)] = struct{}{}
	}

	merged := resource.DeepCopyValue(payloadObject)
	for _, pointer := range slices.Sorted(maps.Keys(derivedFields)) {
		if _, allowed := allowedPointers[pointer]; !allowed {
			continue
		}
		if _, found, err := resource.LookupJSONPointer(merged, pointer); err == nil && found {
			continue
		}
		next, err := resource.SetJSONPointerValue(merged, pointer, derivedFields[pointer])
		if err != nil {
			continue
		}
		merged = next
	}

	return merged
}

func validateValueAgainstOpenAPISchema(
	value any,
	schema any,
	document map[string]any,
	location string,
	visitedRefs map[string]struct{},
	extraKnownProperties map[string]struct{},
	depth int,
) error {
	if depth > maxSchemaDepth {
		return fmt.Errorf("%s schema nesting exceeds supported depth", location)
	}

	resolvedSchema, err := resolveSchemaValue(document, schema, visitedRefs, depth)
	if err != nil {
		return fmt.Errorf("%s %w", location, err)
	}
	if len(resolvedSchema) == 0 {
		return nil
	}

	if nullableSchema(resolvedSchema) && value == nil {
		return nil
	}

	if err := validateSchemaCombiners(value, resolvedSchema, document, location, extraKnownProperties, depth); err != nil {
		return err
	}
	if err := validateSchemaEnum(value, resolvedSchema, location); err != nil {
		return err
	}
	if err := validateSchemaType(value, resolvedSchema, location); err != nil {
		return err
	}
	if err := validateSchemaObject(value, resolvedSchema, document, location, extraKnownProperties, depth); err != nil {
		return err
	}
	if err := validateSchemaArray(value, resolvedSchema, document, location, depth); err != nil {
		return err
	}

	return nil
}

func nullableSchema(schema map[string]any) bool {
	nullable, ok := schema["nullable"].(bool)
	return ok && nullable
}

func validateSchemaCombiners(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	extraKnownProperties map[string]struct{},
	depth int,
) error {
	if allOf, ok := schemaSlice(schema["allOf"]); ok && len(allOf) > 0 {
		unionFields := map[string]struct{}{}
		for key := range extraKnownProperties {
			unionFields[key] = struct{}{}
		}
		if ownProperties, ok := asStringAnyMap(schema["properties"]); ok {
			for key := range ownProperties {
				unionFields[key] = struct{}{}
			}
		}
		for _, name := range requiredPropertyNames(schema["required"]) {
			unionFields[name] = struct{}{}
		}
		for _, item := range allOf {
			branchFields := topLevelSchemaObjectFieldNames(item, document, map[string]struct{}{}, depth+1)
			for key := range branchFields {
				unionFields[key] = struct{}{}
			}
		}
		for idx, item := range allOf {
			if err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s allOf[%d]", location, idx),
				map[string]struct{}{},
				unionFields,
				depth+1,
			); err != nil {
				return err
			}
		}
	}

	if anyOf, ok := schemaSlice(schema["anyOf"]); ok && len(anyOf) > 0 {
		anyMatched := false
		var firstErr error
		for idx, item := range anyOf {
			err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s anyOf[%d]", location, idx),
				map[string]struct{}{},
				nil,
				depth+1,
			)
			if err == nil {
				anyMatched = true
				break
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if !anyMatched {
			if firstErr != nil {
				return firstErr
			}
			return fmt.Errorf("%s did not match any schema in anyOf", location)
		}
	}

	if oneOf, ok := schemaSlice(schema["oneOf"]); ok && len(oneOf) > 0 {
		matches := 0
		for idx, item := range oneOf {
			err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s oneOf[%d]", location, idx),
				map[string]struct{}{},
				nil,
				depth+1,
			)
			if err == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%s expected exactly one oneOf schema match, got %d", location, matches)
		}
	}

	return nil
}

func validateSchemaEnum(value any, schema map[string]any, location string) error {
	enumValues, ok := schemaSlice(schema["enum"])
	if !ok || len(enumValues) == 0 {
		return nil
	}

	for _, candidate := range enumValues {
		if reflect.DeepEqual(candidate, value) {
			return nil
		}
	}
	return fmt.Errorf("%s value is not allowed by enum", location)
}

func validateSchemaType(value any, schema map[string]any, location string) error {
	types := schemaTypeNames(schema["type"])
	if len(types) == 0 {
		return nil
	}

	for _, schemaType := range types {
		if valueMatchesSchemaType(value, schemaType) {
			return nil
		}
	}

	return fmt.Errorf(
		"%s expected type [%s], got %s",
		location,
		strings.Join(types, ", "),
		describeValueType(value),
	)
}

func validateSchemaObject(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	extraKnownProperties map[string]struct{},
	depth int,
) error {
	properties, hasProperties := asStringAnyMap(schema["properties"])
	required := requiredPropertyNames(schema["required"])
	_, hasAdditional := schema["additionalProperties"]
	expectsObject := hasProperties || len(required) > 0 || hasAdditional || schemaTypeContains(schema["type"], "object")
	if !expectsObject {
		return nil
	}

	objectValue, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s expected object, got %s", location, describeValueType(value))
	}

	for _, propertyName := range required {
		if _, exists := objectValue[propertyName]; !exists {
			return fmt.Errorf("%s missing required property %q", location, propertyName)
		}
	}

	for propertyName, propertySchema := range properties {
		propertyValue, exists := objectValue[propertyName]
		if !exists {
			continue
		}
		if err := validateValueAgainstOpenAPISchema(
			propertyValue,
			propertySchema,
			document,
			dotPath(location, propertyName),
			map[string]struct{}{},
			nil,
			depth+1,
		); err != nil {
			return err
		}
	}

	if additionalValue, hasAdditional := schema["additionalProperties"]; hasAdditional {
		ownAllOfFields := map[string]struct{}{}
		if allOf, ok := schemaSlice(schema["allOf"]); ok {
			for _, item := range allOf {
				for key := range topLevelSchemaObjectFieldNames(item, document, map[string]struct{}{}, depth+1) {
					ownAllOfFields[key] = struct{}{}
				}
			}
		}
		switch typed := additionalValue.(type) {
		case bool:
			if !typed {
				for key := range objectValue {
					if _, known := properties[key]; known {
						continue
					}
					if _, known := extraKnownProperties[key]; known {
						continue
					}
					if _, known := ownAllOfFields[key]; known {
						continue
					}
					return fmt.Errorf("%s property %q is not allowed", location, key)
				}
			}
		default:
			for key, propertyValue := range objectValue {
				if _, known := properties[key]; known {
					continue
				}
				if _, known := extraKnownProperties[key]; known {
					continue
				}
				if _, known := ownAllOfFields[key]; known {
					continue
				}
				if err := validateValueAgainstOpenAPISchema(
					propertyValue,
					typed,
					document,
					dotPath(location, key),
					map[string]struct{}{},
					nil,
					depth+1,
				); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateSchemaArray(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	depth int,
) error {
	itemsValue, hasItems := schema["items"]
	expectsArray := hasItems || schemaTypeContains(schema["type"], "array")
	if !expectsArray {
		return nil
	}

	arrayValue, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s expected array, got %s", location, describeValueType(value))
	}

	if !hasItems {
		return nil
	}

	if tupleItems, ok := schemaSlice(itemsValue); ok {
		for idx := 0; idx < len(tupleItems) && idx < len(arrayValue); idx++ {
			if err := validateValueAgainstOpenAPISchema(
				arrayValue[idx],
				tupleItems[idx],
				document,
				fmt.Sprintf("%s[%d]", location, idx),
				map[string]struct{}{},
				nil,
				depth+1,
			); err != nil {
				return err
			}
		}
		return nil
	}

	for idx, item := range arrayValue {
		if err := validateValueAgainstOpenAPISchema(
			item,
			itemsValue,
			document,
			fmt.Sprintf("%s[%d]", location, idx),
			map[string]struct{}{},
			nil,
			depth+1,
		); err != nil {
			return err
		}
	}

	return nil
}
