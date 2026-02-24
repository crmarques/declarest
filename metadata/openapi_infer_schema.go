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
