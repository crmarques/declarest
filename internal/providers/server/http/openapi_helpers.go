package http

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/metadata"
)

func findOpenAPIPathItem(document map[string]any, requestPath string) (string, map[string]any, bool) {
	pathsValue, ok := document["paths"]
	if !ok {
		return "", nil, false
	}
	paths, ok := normalizeDynamicValue(pathsValue).(map[string]any)
	if !ok {
		return "", nil, false
	}

	normalizedRequest := normalizeRequestPath(requestPath)
	if normalizedRequest == "" {
		return "", nil, false
	}

	if direct, ok := paths[normalizedRequest]; ok {
		if pathItem, ok := normalizeDynamicValue(direct).(map[string]any); ok {
			return normalizedRequest, pathItem, true
		}
	}

	type match struct {
		pathKey      string
		pathItem     map[string]any
		templateVars int
	}

	candidates := make([]match, 0)
	for key, value := range paths {
		pathItem, ok := normalizeDynamicValue(value).(map[string]any)
		if !ok {
			continue
		}
		templateVars, matches := openAPIPathMatches(key, normalizedRequest)
		if !matches {
			continue
		}
		candidates = append(candidates, match{
			pathKey:      key,
			pathItem:     pathItem,
			templateVars: templateVars,
		})
	}

	if len(candidates) == 0 {
		return "", nil, false
	}

	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].templateVars != candidates[j].templateVars {
			return candidates[i].templateVars < candidates[j].templateVars
		}
		return candidates[i].pathKey < candidates[j].pathKey
	})
	best := candidates[0]
	return best.pathKey, best.pathItem, true
}

func openAPIPathMatches(templatePath string, requestPath string) (int, bool) {
	template := normalizeRequestPath(templatePath)
	request := normalizeRequestPath(requestPath)
	if template == request {
		return 0, true
	}

	templateSegments := splitPathSegments(template)
	requestSegments := splitPathSegments(request)
	if len(templateSegments) != len(requestSegments) {
		return 0, false
	}

	templateVars := 0
	for idx, segment := range templateSegments {
		if isOpenAPIPathVariable(segment) {
			templateVars++
			continue
		}
		if segment != requestSegments[idx] {
			return 0, false
		}
	}
	return templateVars, true
}

func splitPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func isOpenAPIPathVariable(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && len(segment) > 2
}

func openAPIPathMethod(pathItem map[string]any, method string) (map[string]any, bool) {
	value, ok := pathItem[strings.ToLower(strings.TrimSpace(method))]
	if !ok {
		return nil, false
	}
	operation, ok := normalizeDynamicValue(value).(map[string]any)
	return operation, ok
}

func inferMethodFromPathItem(pathItem map[string]any, operation metadata.Operation) string {
	preferred := preferredMethodsForOperation(operation)
	for _, method := range preferred {
		if _, ok := openAPIPathMethod(pathItem, method); ok {
			return strings.ToUpper(method)
		}
	}
	return ""
}

func preferredMethodsForOperation(operation metadata.Operation) []string {
	switch operation {
	case metadata.OperationCreate:
		return []string{"post", "put"}
	case metadata.OperationUpdate:
		return []string{"put", "patch", "post"}
	case metadata.OperationDelete:
		return []string{"delete"}
	case metadata.OperationList, metadata.OperationGet, metadata.OperationCompare:
		return []string{"get"}
	default:
		return []string{"get", "post", "put", "patch", "delete"}
	}
}

func inferAcceptContentType(operation map[string]any) string {
	responses, ok := normalizeDynamicValue(operation["responses"]).(map[string]any)
	if !ok {
		return ""
	}

	preferredStatus := []string{"200", "201", "202", "default"}
	for _, status := range preferredStatus {
		if contentType := contentTypeFromResponseEntry(responses[status]); contentType != "" {
			return contentType
		}
	}

	statusCodes := make([]string, 0, len(responses))
	for status := range responses {
		statusCodes = append(statusCodes, status)
	}
	sort.Strings(statusCodes)
	for _, status := range statusCodes {
		if contentType := contentTypeFromResponseEntry(responses[status]); contentType != "" {
			return contentType
		}
	}
	return ""
}

func contentTypeFromResponseEntry(entry any) string {
	response, ok := normalizeDynamicValue(entry).(map[string]any)
	if !ok {
		return ""
	}
	content, ok := normalizeDynamicValue(response["content"]).(map[string]any)
	if !ok || len(content) == 0 {
		return ""
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}

func inferRequestContentType(operation map[string]any) string {
	requestBody, ok := normalizeDynamicValue(operation["requestBody"]).(map[string]any)
	if !ok {
		return ""
	}
	content, ok := normalizeDynamicValue(requestBody["content"]).(map[string]any)
	if !ok || len(content) == 0 {
		return ""
	}

	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys[0]
}

func normalizeDynamicValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeDynamicValue(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprint(key)] = normalizeDynamicValue(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for idx, item := range typed {
			out[idx] = normalizeDynamicValue(item)
		}
		return out
	default:
		return typed
	}
}

func cloneValue(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}

	var cloned any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return value
	}
	return cloned
}
