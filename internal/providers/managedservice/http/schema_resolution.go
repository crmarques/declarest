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
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
)

const maxSchemaDepth = 96

func (g *Client) resolveOpenAPISchemaForValidation(
	ctx context.Context,
	requestPath string,
	requestMethod string,
	schemaRef string,
) (any, map[string]any, error) {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil, nil, faults.Invalid(
			"validate.schemaRef requires managed-service.http.openapi to be configured",
			nil,
		)
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return nil, nil, err
	}

	if schemaRef == "openapi:request-body" {
		_, pathItem, found := findOpenAPIPathItem(document, requestPath)
		if !found {
			return nil, nil, faults.Invalid(
				fmt.Sprintf("OpenAPI path %q was not found for validate.schemaRef", requestPath),
				nil,
			)
		}

		method := strings.ToUpper(strings.TrimSpace(requestMethod))
		if method == "" {
			return nil, nil, faults.Invalid("request method is required for OpenAPI request-body validation", nil)
		}
		operationItem, found := openAPIPathMethod(pathItem, method)
		if !found {
			return nil, nil, faults.Invalid(
				fmt.Sprintf(
					"OpenAPI path %q does not support method %s for validate.schemaRef",
					requestPath,
					method,
				),
				nil,
			)
		}

		schema, found := openAPIRequestBodySchemaForValidation(document, operationItem)
		if !found {
			return nil, nil, faults.Invalid(
				fmt.Sprintf(
					"OpenAPI request body schema was not found for %s %q",
					method,
					requestPath,
				),
				nil,
			)
		}
		return schema, document, nil
	}

	if strings.HasPrefix(schemaRef, "openapi:#/") {
		ref := strings.TrimPrefix(schemaRef, "openapi:")
		resolved, found := resolveOpenAPIJSONPointer(document, ref)
		if !found {
			return nil, nil, faults.Invalid(
				fmt.Sprintf("OpenAPI schema reference %q could not be resolved", schemaRef),
				nil,
			)
		}
		return resolved, document, nil
	}

	return nil, nil, faults.Invalid(
		fmt.Sprintf("validate.schemaRef %q is not supported", schemaRef),
		nil,
	)
}

func openAPIRequestBodySchemaForValidation(document map[string]any, operation map[string]any) (any, bool) {
	requestBodyValue, found := operation["requestBody"]
	if !found {
		return nil, false
	}

	resolvedRequestBody, ok := resolveOpenAPIValueRef(document, requestBodyValue, map[string]struct{}{}, 0)
	if !ok {
		return nil, false
	}

	requestBody, ok := asStringAnyMap(resolvedRequestBody)
	if !ok {
		return nil, false
	}

	contentValue, found := requestBody["content"]
	if !found {
		return nil, false
	}
	content, ok := asStringAnyMap(contentValue)
	if !ok || len(content) == 0 {
		return nil, false
	}

	mediaTypes := make([]string, 0, len(content))
	for mediaType := range content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Slice(mediaTypes, func(i int, j int) bool {
		leftScore := openAPIMediaTypePriority(mediaTypes[i])
		rightScore := openAPIMediaTypePriority(mediaTypes[j])
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		return mediaTypes[i] < mediaTypes[j]
	})

	for _, mediaType := range mediaTypes {
		mediaValue, ok := asStringAnyMap(content[mediaType])
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

func openAPIMediaTypePriority(value string) int {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "application/json":
		return 0
	case strings.HasPrefix(normalized, "application/") && strings.HasSuffix(normalized, "+json"):
		return 1
	default:
		return 2
	}
}

func resolveOpenAPIValueRef(
	document map[string]any,
	value any,
	visited map[string]struct{},
	depth int,
) (any, bool) {
	if depth > maxSchemaDepth {
		return nil, false
	}

	mapped, ok := asStringAnyMap(value)
	if !ok {
		return value, true
	}

	refValue, hasRef := mapped["$ref"]
	if !hasRef {
		return mapped, true
	}

	ref, ok := refValue.(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, false
	}
	ref = strings.TrimSpace(ref)
	if _, exists := visited[ref]; exists {
		return nil, false
	}

	resolved, found := resolveOpenAPIJSONPointer(document, ref)
	if !found {
		return nil, false
	}

	visited[ref] = struct{}{}
	finalValue, ok := resolveOpenAPIValueRef(document, resolved, visited, depth+1)
	delete(visited, ref)
	return finalValue, ok
}

func resolveOpenAPIJSONPointer(document map[string]any, ref string) (any, bool) {
	trimmedRef := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmedRef, "#/") {
		return nil, false
	}

	pointer := strings.TrimPrefix(trimmedRef, "#/")
	if strings.TrimSpace(pointer) == "" {
		return document, true
	}

	current := any(document)
	segments := strings.Split(pointer, "/")
	for _, rawSegment := range segments {
		segment := strings.ReplaceAll(strings.ReplaceAll(rawSegment, "~1", "/"), "~0", "~")
		currentMap, ok := asStringAnyMap(current)
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

func resolveSchemaValue(
	document map[string]any,
	schema any,
	visitedRefs map[string]struct{},
	depth int,
) (map[string]any, error) {
	if depth > maxSchemaDepth {
		return nil, fmt.Errorf("schema reference depth exceeded")
	}

	schemaMap, ok := asStringAnyMap(schema)
	if !ok {
		return nil, nil
	}

	refValue, hasRef := schemaMap["$ref"]
	if !hasRef {
		return schemaMap, nil
	}

	ref, ok := refValue.(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("schema $ref must be a non-empty string")
	}
	ref = strings.TrimSpace(ref)
	if _, exists := visitedRefs[ref]; exists {
		return nil, fmt.Errorf("schema reference cycle detected for %q", ref)
	}

	resolved, found := resolveOpenAPIJSONPointer(document, ref)
	if !found {
		return nil, fmt.Errorf("schema reference %q was not found", ref)
	}

	visitedRefs[ref] = struct{}{}
	resolvedSchema, err := resolveSchemaValue(document, resolved, visitedRefs, depth+1)
	delete(visitedRefs, ref)
	if err != nil {
		return nil, err
	}
	return resolvedSchema, nil
}
