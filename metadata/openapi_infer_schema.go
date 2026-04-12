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

package metadata

import (
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/resource"
)

func openAPIPathItems(openAPISpec any) map[string]map[string]any {
	root, ok := asStringMap(openAPISpec)
	if !ok {
		return nil
	}

	pathsValue, ok := root["paths"]
	if !ok {
		return nil
	}
	pathsMap, ok := asStringMap(pathsValue)
	if !ok {
		return nil
	}

	result := make(map[string]map[string]any, len(pathsMap))
	keys := make([]string, 0, len(pathsMap))
	for pathKey := range pathsMap {
		keys = append(keys, pathKey)
	}
	sort.Strings(keys)

	for _, pathKey := range keys {
		pathItem, ok := asStringMap(pathsMap[pathKey])
		if !ok {
			continue
		}
		normalizedPath := path.Clean("/" + strings.Trim(pathKey, "/"))
		result[normalizedPath] = pathItem
	}

	return result
}

func inferOpenAPIResponseAttributes(
	candidate openAPICandidate,
	pathItems map[string]map[string]any,
	openAPISpec any,
) map[string]struct{} {
	if candidate.path == "" || len(pathItems) == 0 {
		return nil
	}

	pathItem, found := pathItems[candidate.path]
	if !found {
		return nil
	}

	for _, method := range []string{"get", "put", "patch", "post"} {
		if !hasOpenAPIMethod(candidate.methods, method) {
			continue
		}

		operationValue, found := pathItem[method]
		if !found {
			continue
		}
		operationItem, ok := asStringMap(operationValue)
		if !ok {
			continue
		}

		attributes := inferOpenAPIOperationResponseAttributes(operationItem, openAPISpec, map[string]struct{}{}, 0)
		if len(attributes) > 0 {
			return attributes
		}
	}

	return nil
}

func inferOpenAPIOperationResponseAttributes(
	operation map[string]any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > 24 {
		return nil
	}

	responsesValue, found := operation["responses"]
	if !found {
		return nil
	}
	responses, ok := asStringMap(responsesValue)
	if !ok {
		return nil
	}

	visitedStatuses := make(map[string]struct{}, len(responses))
	for _, status := range []string{"200", "201", "202", "default"} {
		entry, found := responses[status]
		if !found {
			continue
		}
		visitedStatuses[status] = struct{}{}
		attributes := inferOpenAPIResponseEntryAttributes(entry, openAPISpec, visitedRefs, depth+1)
		if len(attributes) > 0 {
			return attributes
		}
	}

	statuses := make([]string, 0, len(responses))
	for status := range responses {
		if _, handled := visitedStatuses[status]; handled {
			continue
		}
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)

	for _, status := range statuses {
		attributes := inferOpenAPIResponseEntryAttributes(responses[status], openAPISpec, visitedRefs, depth+1)
		if len(attributes) > 0 {
			return attributes
		}
	}

	return nil
}

func inferOpenAPIResponseEntryAttributes(
	entry any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > 24 {
		return nil
	}

	response, ok := asStringMap(entry)
	if !ok {
		return nil
	}

	contentValue, found := response["content"]
	if !found {
		return nil
	}
	content, ok := asStringMap(contentValue)
	if !ok {
		return nil
	}

	mediaTypes := make([]string, 0, len(content))
	for mediaType := range content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Strings(mediaTypes)

	for _, mediaType := range mediaTypes {
		mediaTypeValue, ok := asStringMap(content[mediaType])
		if !ok {
			continue
		}
		schemaValue, found := mediaTypeValue["schema"]
		if !found {
			continue
		}

		attributes := inferOpenAPISchemaAttributes(schemaValue, openAPISpec, visitedRefs, depth+1)
		if len(attributes) > 0 {
			return attributes
		}
	}

	return nil
}

func inferOpenAPISchemaAttributes(
	schema any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > 24 {
		return nil
	}

	schemaMap, ok := asStringMap(schema)
	if !ok {
		return nil
	}

	if refValue, hasRef := schemaMap["$ref"]; hasRef {
		ref, ok := refValue.(string)
		if ok {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				if _, isVisited := visitedRefs[ref]; isVisited {
					return nil
				}
				resolved, found := resolveOpenAPIRef(openAPISpec, ref)
				if found {
					visitedRefs[ref] = struct{}{}
					resolvedAttributes := inferOpenAPISchemaAttributes(resolved, openAPISpec, visitedRefs, depth+1)
					delete(visitedRefs, ref)
					if len(resolvedAttributes) > 0 {
						return resolvedAttributes
					}
				}
			}
		}
	}

	merged := map[string]struct{}{}
	for _, combiner := range []string{"allOf", "oneOf", "anyOf"} {
		combinedValue, found := schemaMap[combiner]
		if !found {
			continue
		}
		entries, ok := combinedValue.([]any)
		if !ok {
			continue
		}
		for _, entry := range entries {
			mergeAttributeSets(merged, inferOpenAPISchemaAttributes(entry, openAPISpec, visitedRefs, depth+1))
		}
	}

	if propertiesValue, found := schemaMap["properties"]; found {
		properties, ok := asStringMap(propertiesValue)
		if ok {
			for key := range properties {
				trimmedKey := strings.TrimSpace(key)
				if trimmedKey == "" {
					continue
				}
				merged[trimmedKey] = struct{}{}
			}
		}
	}
	if len(merged) > 0 {
		return merged
	}

	if itemsValue, found := schemaMap["items"]; found {
		itemsAttributes := inferOpenAPISchemaAttributes(itemsValue, openAPISpec, visitedRefs, depth+1)
		if len(itemsAttributes) > 0 {
			return itemsAttributes
		}
	}

	return nil
}

func resolveOpenAPIRef(openAPISpec any, ref string) (any, bool) {
	trimmedRef := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmedRef, "#/") {
		return nil, false
	}

	pointer := strings.TrimPrefix(trimmedRef, "#/")
	if strings.TrimSpace(pointer) == "" {
		return openAPISpec, true
	}

	current := openAPISpec
	segments := strings.Split(pointer, "/")
	for _, rawSegment := range segments {
		segment := strings.ReplaceAll(strings.ReplaceAll(rawSegment, "~1", "/"), "~0", "~")
		currentMap, ok := asStringMap(current)
		if !ok {
			return nil, false
		}
		next, found := currentMap[segment]
		if !found {
			return nil, false
		}
		current = next
	}

	return current, true
}

func mergeAttributeSets(target map[string]struct{}, source map[string]struct{}) {
	if len(source) == 0 {
		return
	}
	for key := range source {
		target[key] = struct{}{}
	}
}

func inferOpenAPIOperationValidationSpec(
	candidate openAPICandidate,
	method string,
	pathItems map[string]map[string]any,
	openAPISpec any,
) *OperationValidationSpec {
	if candidate.path == "" || len(pathItems) == 0 {
		return nil
	}

	pathItem, found := pathItems[candidate.path]
	if !found {
		return nil
	}

	operationValue, found := pathItem[strings.ToLower(strings.TrimSpace(method))]
	if !found {
		return nil
	}
	operationItem, ok := asStringMap(operationValue)
	if !ok {
		return nil
	}

	requestBodySchema, payloadType, found := inferOpenAPIRequestBodySchema(operationItem, openAPISpec)
	if !found || !resource.IsStructuredPayloadType(payloadType) {
		return nil
	}

	spec := &OperationValidationSpec{
		SchemaRef: "openapi:request-body",
	}

	required := inferOpenAPISchemaRequiredAttributes(requestBodySchema, openAPISpec, map[string]struct{}{}, 0)
	if len(required) > 0 {
		names := make([]string, 0, len(required))
		for key := range required {
			names = append(names, resource.JSONPointerForObjectKey(key))
		}
		sort.Strings(names)
		spec.RequiredAttributes = names
	}

	return spec
}

func inferOpenAPIRequestBodySchema(operation map[string]any, openAPISpec any) (any, string, bool) {
	requestBodyValue, found := operation["requestBody"]
	if !found {
		return nil, "", false
	}

	requestBody, ok := resolveOpenAPIValueRefForInference(openAPISpec, requestBodyValue, map[string]struct{}{}, 0)
	if !ok {
		return nil, "", false
	}
	requestBodyMap, ok := asStringMap(requestBody)
	if !ok {
		return nil, "", false
	}

	contentValue, found := requestBodyMap["content"]
	if !found {
		return nil, "", false
	}
	content, ok := asStringMap(contentValue)
	if !ok || len(content) == 0 {
		return nil, "", false
	}

	candidate, found := selectOpenAPIContentCandidate(content, openAPISpec)
	if !found || candidate.schema == nil {
		return nil, "", false
	}

	return candidate.schema, candidate.payloadType, true
}

func inferOpenAPISchemaRequiredAttributes(
	schema any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > 24 {
		return nil
	}

	schemaValue, ok := resolveOpenAPIValueRefForInference(openAPISpec, schema, visitedRefs, depth+1)
	if !ok {
		return nil
	}
	schemaMap, ok := asStringMap(schemaValue)
	if !ok {
		return nil
	}

	required := map[string]struct{}{}
	if requiredValue, found := schemaMap["required"]; found {
		requiredNames, ok := requiredValue.([]any)
		if ok {
			for _, nameValue := range requiredNames {
				name, ok := nameValue.(string)
				if !ok {
					continue
				}
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				required[trimmed] = struct{}{}
			}
		}
	}

	if allOfValue, found := schemaMap["allOf"]; found {
		allOfItems, ok := allOfValue.([]any)
		if ok {
			for _, item := range allOfItems {
				mergeAttributeSets(
					required,
					inferOpenAPISchemaRequiredAttributes(item, openAPISpec, visitedRefs, depth+1),
				)
			}
		}
	}

	if len(required) == 0 {
		return nil
	}
	return required
}

func resolveOpenAPIValueRefForInference(
	openAPISpec any,
	value any,
	visitedRefs map[string]struct{},
	depth int,
) (any, bool) {
	if depth > 24 {
		return nil, false
	}

	valueMap, ok := asStringMap(value)
	if !ok {
		return value, true
	}

	refValue, hasRef := valueMap["$ref"]
	if !hasRef {
		return valueMap, true
	}

	ref, ok := refValue.(string)
	if !ok {
		return nil, false
	}
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return nil, false
	}
	if _, seen := visitedRefs[trimmedRef]; seen {
		return nil, false
	}

	resolved, found := resolveOpenAPIRef(openAPISpec, trimmedRef)
	if !found {
		return nil, false
	}

	visitedRefs[trimmedRef] = struct{}{}
	nextValue, ok := resolveOpenAPIValueRefForInference(openAPISpec, resolved, visitedRefs, depth+1)
	delete(visitedRefs, trimmedRef)
	return nextValue, ok
}

type openAPIContentCandidate struct {
	mediaType   string
	schema      any
	payloadType string
	priority    int
}

func inferOpenAPIOperationPayloadTypes(operation map[string]any, openAPISpec any) []string {
	seen := map[string]struct{}{}
	if payloadType, ok := inferOpenAPIRequestBodyPayloadType(operation, openAPISpec); ok {
		seen[payloadType] = struct{}{}
	}
	if payloadType, ok := inferOpenAPIResponsePayloadType(operation, openAPISpec); ok {
		seen[payloadType] = struct{}{}
	}

	if len(seen) == 0 {
		return nil
	}

	items := make([]string, 0, len(seen))
	for payloadType := range seen {
		items = append(items, payloadType)
	}
	sort.Strings(items)
	return items
}

func inferOpenAPIRequestBodyPayloadType(operation map[string]any, openAPISpec any) (string, bool) {
	requestBodyValue, found := operation["requestBody"]
	if !found {
		return "", false
	}

	requestBody, ok := resolveOpenAPIValueRefForInference(openAPISpec, requestBodyValue, map[string]struct{}{}, 0)
	if !ok {
		return "", false
	}
	requestBodyMap, ok := asStringMap(requestBody)
	if !ok {
		return "", false
	}

	contentValue, found := requestBodyMap["content"]
	if !found {
		return "", false
	}
	content, ok := asStringMap(contentValue)
	if !ok || len(content) == 0 {
		return "", false
	}

	candidate, found := selectOpenAPIContentCandidate(content, openAPISpec)
	if !found || strings.TrimSpace(candidate.payloadType) == "" {
		return "", false
	}
	return candidate.payloadType, true
}

func inferOpenAPIResponsePayloadType(operation map[string]any, openAPISpec any) (string, bool) {
	responsesValue, found := operation["responses"]
	if !found {
		return "", false
	}
	responses, ok := asStringMap(responsesValue)
	if !ok {
		return "", false
	}

	visitedStatuses := make(map[string]struct{}, len(responses))
	for _, status := range []string{"200", "201", "202", "default"} {
		entry, found := responses[status]
		if !found {
			continue
		}
		visitedStatuses[status] = struct{}{}
		if payloadType, ok := inferOpenAPIResponseEntryPayloadType(entry, openAPISpec, map[string]struct{}{}, 0); ok {
			return payloadType, true
		}
	}

	statuses := make([]string, 0, len(responses))
	for status := range responses {
		if _, handled := visitedStatuses[status]; handled {
			continue
		}
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		if payloadType, ok := inferOpenAPIResponseEntryPayloadType(responses[status], openAPISpec, map[string]struct{}{}, 0); ok {
			return payloadType, true
		}
	}

	return "", false
}

func inferOpenAPIResponseEntryPayloadType(
	entry any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) (string, bool) {
	if depth > 24 {
		return "", false
	}

	responseValue, ok := resolveOpenAPIValueRefForInference(openAPISpec, entry, visitedRefs, depth+1)
	if !ok {
		return "", false
	}
	response, ok := asStringMap(responseValue)
	if !ok {
		return "", false
	}

	contentValue, found := response["content"]
	if !found {
		return "", false
	}
	content, ok := asStringMap(contentValue)
	if !ok || len(content) == 0 {
		return "", false
	}

	candidate, found := selectOpenAPIContentCandidate(content, openAPISpec)
	if !found || strings.TrimSpace(candidate.payloadType) == "" {
		return "", false
	}
	return candidate.payloadType, true
}

func selectOpenAPIContentCandidate(content map[string]any, openAPISpec any) (openAPIContentCandidate, bool) {
	candidates := make([]openAPIContentCandidate, 0, len(content))
	for mediaType, rawValue := range content {
		mediaValue, ok := asStringMap(rawValue)
		if !ok {
			continue
		}

		schemaValue := mediaValue["schema"]
		payloadType, _ := inferOpenAPIContentPayloadType(mediaType, schemaValue, openAPISpec)
		candidates = append(candidates, openAPIContentCandidate{
			mediaType:   mediaType,
			schema:      schemaValue,
			payloadType: payloadType,
			priority:    openAPIMediaTypePriority(mediaType, schemaValue, openAPISpec),
		})
	}

	if len(candidates) == 0 {
		return openAPIContentCandidate{}, false
	}

	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].mediaType < candidates[j].mediaType
	})

	return candidates[0], true
}

func inferOpenAPIContentPayloadType(mediaType string, schema any, openAPISpec any) (string, bool) {
	if isOpenAPIBinarySchema(schema, openAPISpec, map[string]struct{}{}, 0) {
		return resource.PayloadTypeOctetStream, true
	}
	return resource.PayloadTypeForMediaType(mediaType)
}

func openAPIMediaTypePriority(mediaType string, schema any, openAPISpec any) int {
	payloadType, ok := inferOpenAPIContentPayloadType(mediaType, schema, openAPISpec)
	if !ok {
		return 6
	}

	switch payloadType {
	case resource.PayloadTypeJSON:
		return 0
	case resource.PayloadTypeYAML:
		return 1
	case resource.PayloadTypeXML:
		return 2
	case resource.PayloadTypeHCL, resource.PayloadTypeINI, resource.PayloadTypeProperties, resource.PayloadTypeText:
		return 3
	case resource.PayloadTypeOctetStream:
		return 4
	default:
		return 5
	}
}

func isOpenAPIBinarySchema(
	schema any,
	openAPISpec any,
	visitedRefs map[string]struct{},
	depth int,
) bool {
	if depth > 24 {
		return false
	}

	schemaValue, ok := resolveOpenAPIValueRefForInference(openAPISpec, schema, visitedRefs, depth+1)
	if !ok {
		return false
	}
	schemaMap, ok := asStringMap(schemaValue)
	if !ok {
		return false
	}

	if strings.EqualFold(strings.TrimSpace(asString(schemaMap["type"])), "string") &&
		strings.EqualFold(strings.TrimSpace(asString(schemaMap["format"])), "binary") {
		return true
	}

	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		rawEntries, found := schemaMap[key]
		if !found {
			continue
		}
		entries, ok := rawEntries.([]any)
		if !ok {
			continue
		}
		for _, entry := range entries {
			if isOpenAPIBinarySchema(entry, openAPISpec, visitedRefs, depth+1) {
				return true
			}
		}
	}

	if itemsValue, found := schemaMap["items"]; found {
		return isOpenAPIBinarySchema(itemsValue, openAPISpec, visitedRefs, depth+1)
	}

	return false
}

func asString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
