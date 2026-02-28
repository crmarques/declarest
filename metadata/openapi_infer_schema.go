package metadata

import (
	"path"
	"sort"
	"strings"
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

	requestBodySchema, found := inferOpenAPIRequestBodySchema(operationItem, openAPISpec)
	if !found {
		return nil
	}

	spec := &OperationValidationSpec{
		SchemaRef: "openapi:request-body",
	}

	required := inferOpenAPISchemaRequiredAttributes(requestBodySchema, openAPISpec, map[string]struct{}{}, 0)
	if len(required) > 0 {
		names := make([]string, 0, len(required))
		for key := range required {
			names = append(names, key)
		}
		sort.Strings(names)
		spec.RequiredAttributes = names
	}

	return spec
}

func inferOpenAPIRequestBodySchema(operation map[string]any, openAPISpec any) (any, bool) {
	requestBodyValue, found := operation["requestBody"]
	if !found {
		return nil, false
	}

	requestBody, ok := resolveOpenAPIValueRefForInference(openAPISpec, requestBodyValue, map[string]struct{}{}, 0)
	if !ok {
		return nil, false
	}
	requestBodyMap, ok := asStringMap(requestBody)
	if !ok {
		return nil, false
	}

	contentValue, found := requestBodyMap["content"]
	if !found {
		return nil, false
	}
	content, ok := asStringMap(contentValue)
	if !ok || len(content) == 0 {
		return nil, false
	}

	mediaTypes := make([]string, 0, len(content))
	for mediaType := range content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Slice(mediaTypes, func(i int, j int) bool {
		leftPriority := openAPIMediaTypePriority(mediaTypes[i])
		rightPriority := openAPIMediaTypePriority(mediaTypes[j])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return mediaTypes[i] < mediaTypes[j]
	})

	for _, mediaType := range mediaTypes {
		mediaValue, ok := asStringMap(content[mediaType])
		if !ok {
			continue
		}
		schemaValue, hasSchema := mediaValue["schema"]
		if !hasSchema {
			continue
		}
		return schemaValue, true
	}

	return nil, false
}

func openAPIMediaTypePriority(mediaType string) int {
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case normalized == "application/json":
		return 0
	case strings.HasPrefix(normalized, "application/") && strings.HasSuffix(normalized, "+json"):
		return 1
	default:
		return 2
	}
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
