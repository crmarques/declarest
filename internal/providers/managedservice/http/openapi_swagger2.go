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
	"maps"
	"slices"
	"strings"
)

func normalizeOpenAPIDocument(document map[string]any) map[string]any {
	if len(document) == 0 || !isSwagger2Document(document) {
		return document
	}

	augmentSwagger2Components(document)
	augmentSwagger2PathOperations(document)
	return document
}

func isSwagger2Document(document map[string]any) bool {
	raw, exists := document["swagger"]
	if !exists {
		return false
	}

	switch typed := raw.(type) {
	case string:
		value := strings.TrimSpace(typed)
		return value == "2" || strings.HasPrefix(value, "2.")
	case float64:
		return typed >= 2 && typed < 3
	case float32:
		return typed >= 2 && typed < 3
	case int:
		return typed == 2
	case int64:
		return typed == 2
	default:
		return false
	}
}

func augmentSwagger2Components(document map[string]any) {
	components, _ := asStringAnyMap(document["components"])
	if components == nil {
		components = map[string]any{}
	}

	copySwagger2ComponentIfMissing(components, "schemas", document["definitions"])
	copySwagger2ComponentIfMissing(components, "parameters", document["parameters"])
	copySwagger2ComponentIfMissing(components, "responses", document["responses"])

	if len(components) > 0 {
		document["components"] = components
	}
}

func copySwagger2ComponentIfMissing(components map[string]any, componentKey string, source any) {
	if _, exists := components[componentKey]; exists {
		return
	}

	mapped, ok := asStringAnyMap(source)
	if !ok || len(mapped) == 0 {
		return
	}
	components[componentKey] = mapped
}

func augmentSwagger2PathOperations(document map[string]any) {
	paths, ok := asStringAnyMap(document["paths"])
	if !ok || len(paths) == 0 {
		return
	}

	globalConsumes := openAPIMediaTypeList(document["consumes"])
	globalProduces := openAPIMediaTypeList(document["produces"])

	for _, pathKey := range slices.Sorted(maps.Keys(paths)) {
		pathItem, ok := asStringAnyMap(paths[pathKey])
		if !ok {
			continue
		}

		pathConsumes := mergeOpenAPIMediaTypeList(pathItem["consumes"], globalConsumes)
		pathProduces := mergeOpenAPIMediaTypeList(pathItem["produces"], globalProduces)

		for _, method := range slices.Sorted(maps.Keys(pathItem)) {
			if !isOpenAPIHTTPMethod(method) {
				continue
			}

			operation, ok := asStringAnyMap(pathItem[method])
			if !ok {
				continue
			}

			consumes := mergeOpenAPIMediaTypeList(operation["consumes"], pathConsumes)
			produces := mergeOpenAPIMediaTypeList(operation["produces"], pathProduces)

			augmentSwagger2OperationRequestBody(document, pathItem, operation, consumes)
			augmentSwagger2OperationResponseContent(document, operation, produces)

			pathItem[method] = operation
		}

		paths[pathKey] = pathItem
	}

	document["paths"] = paths
}

func isOpenAPIHTTPMethod(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}

func augmentSwagger2OperationRequestBody(
	document map[string]any,
	pathItem map[string]any,
	operation map[string]any,
	consumes []string,
) {
	if _, exists := operation["requestBody"]; exists {
		return
	}

	schema, hasSchema := swagger2OperationRequestBodySchema(document, pathItem, operation)
	if !hasSchema {
		return
	}

	mediaTypes := effectiveOpenAPIMediaTypes(consumes)
	content := make(map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		content[mediaType] = map[string]any{
			"schema": schema,
		}
	}

	operation["requestBody"] = map[string]any{
		"content": content,
	}
}

func swagger2OperationRequestBodySchema(
	document map[string]any,
	pathItem map[string]any,
	operation map[string]any,
) (any, bool) {
	if schema, found := swagger2RequestBodySchemaFromParameters(document, operation["parameters"]); found {
		return schema, true
	}
	if schema, found := swagger2RequestBodySchemaFromParameters(document, pathItem["parameters"]); found {
		return schema, true
	}
	return nil, false
}

func swagger2RequestBodySchemaFromParameters(document map[string]any, parametersValue any) (any, bool) {
	parameters, ok := schemaSlice(parametersValue)
	if !ok || len(parameters) == 0 {
		return nil, false
	}

	for _, parameterValue := range parameters {
		resolvedParameter, ok := resolveOpenAPIValueRef(document, parameterValue, map[string]struct{}{}, 0)
		if !ok {
			continue
		}

		parameter, ok := asStringAnyMap(resolvedParameter)
		if !ok {
			continue
		}

		inValue, _ := parameter["in"].(string)
		if strings.ToLower(strings.TrimSpace(inValue)) != "body" {
			continue
		}

		schema, hasSchema := parameter["schema"]
		if !hasSchema {
			continue
		}

		return schema, true
	}

	return nil, false
}

func augmentSwagger2OperationResponseContent(document map[string]any, operation map[string]any, produces []string) {
	responses, ok := asStringAnyMap(operation["responses"])
	if !ok || len(responses) == 0 {
		return
	}

	mediaTypes := effectiveOpenAPIMediaTypes(produces)
	for _, status := range slices.Sorted(maps.Keys(responses)) {
		responseValue := responses[status]
		resolvedResponse, ok := resolveOpenAPIValueRef(document, responseValue, map[string]struct{}{}, 0)
		if !ok {
			resolvedResponse = responseValue
		}

		response, ok := asStringAnyMap(resolvedResponse)
		if !ok {
			continue
		}

		if content, hasContent := asStringAnyMap(response["content"]); hasContent && len(content) > 0 {
			continue
		}

		schema, hasSchema := response["schema"]
		content := make(map[string]any, len(mediaTypes))
		for _, mediaType := range mediaTypes {
			media := map[string]any{}
			if hasSchema {
				media["schema"] = schema
			}
			content[mediaType] = media
		}

		response["content"] = content
		responses[status] = response
	}

	operation["responses"] = responses
}

func mergeOpenAPIMediaTypeList(value any, fallback []string) []string {
	if mediaTypes := openAPIMediaTypeList(value); len(mediaTypes) > 0 {
		return mediaTypes
	}
	return cloneStringSlice(fallback)
}

func effectiveOpenAPIMediaTypes(mediaTypes []string) []string {
	if len(mediaTypes) > 0 {
		return cloneStringSlice(mediaTypes)
	}
	return []string{defaultMediaType}
}

func openAPIMediaTypeList(value any) []string {
	seen := map[string]struct{}{}
	mediaTypes := make([]string, 0)

	appendMediaType := func(candidate string) {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		mediaTypes = append(mediaTypes, trimmed)
	}

	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			appendMediaType(text)
		}
	case []string:
		for _, text := range typed {
			appendMediaType(text)
		}
	case string:
		appendMediaType(typed)
	}

	if len(mediaTypes) == 0 {
		return nil
	}
	return mediaTypes
}
