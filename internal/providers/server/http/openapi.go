package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPResourceServerGateway) GetOpenAPISpec(ctx context.Context) (resource.Value, error) {
	doc, err := g.openAPIDocument(ctx)
	if err != nil {
		return nil, err
	}
	return cloneValue(doc), nil
}

func validateOpenAPISource(source string) error {
	value := strings.TrimSpace(source)
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return validationError("managed-server.http.openapi is invalid", err)
	}

	if parsed.Scheme == "" {
		return nil
	}
	if parsed.Scheme != "https" {
		return validationError("managed-server.http.openapi must use https when configured as URL", nil)
	}
	if parsed.Host == "" {
		return validationError("managed-server.http.openapi URL host is required", nil)
	}
	return nil
}

func (g *HTTPResourceServerGateway) openAPIDocument(ctx context.Context) (map[string]any, error) {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil, validationError("managed-server.http.openapi is not configured", nil)
	}

	g.openapiMu.Lock()
	if g.openapiLoaded {
		doc := g.openapiDoc
		err := g.openapiErr
		g.openapiMu.Unlock()
		return doc, err
	}
	g.openapiMu.Unlock()

	document, err := g.loadOpenAPIDocument(ctx)

	g.openapiMu.Lock()
	if !g.openapiLoaded {
		g.openapiDoc = document
		g.openapiErr = err
		g.openapiLoaded = true
	}
	doc := g.openapiDoc
	loadErr := g.openapiErr
	g.openapiMu.Unlock()
	return doc, loadErr
}

func (g *HTTPResourceServerGateway) loadOpenAPIDocument(ctx context.Context) (map[string]any, error) {
	source := strings.TrimSpace(g.openAPISource)
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, validationError("managed-server.http.openapi is invalid", err)
	}

	var content []byte
	switch parsed.Scheme {
	case "":
		content, err = os.ReadFile(source)
		if err != nil {
			return nil, notFoundError("managed-server.http.openapi file could not be read", err)
		}
	case "https":
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, internalError("failed to create OpenAPI request", err)
		}
		if err := g.applyAuth(ctx, request); err != nil {
			return nil, err
		}

		response, err := g.doRequest(ctx, "openapi", request)
		if err != nil {
			return nil, transportError("failed to fetch OpenAPI document", err)
		}
		defer response.Body.Close()

		content, err = io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if err != nil {
			return nil, transportError("failed to read OpenAPI response body", err)
		}
		if response.StatusCode >= http.StatusBadRequest {
			if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
				return nil, authError(
					fmt.Sprintf("OpenAPI request failed with status %d: %s", response.StatusCode, summarizeBody(content)),
					nil,
				)
			}
			if response.StatusCode == http.StatusNotFound {
				return nil, notFoundError(
					fmt.Sprintf("OpenAPI request failed with status %d: %s", response.StatusCode, summarizeBody(content)),
					nil,
				)
			}
			return nil, transportError(
				fmt.Sprintf("OpenAPI request failed with status %d: %s", response.StatusCode, summarizeBody(content)),
				nil,
			)
		}
	default:
		return nil, validationError("managed-server.http.openapi must be a local file path or https URL", nil)
	}

	var root any
	if jsonErr := json.Unmarshal(content, &root); jsonErr != nil {
		if yamlErr := yaml.Unmarshal(content, &root); yamlErr != nil {
			return nil, validationError("OpenAPI document must be valid JSON or YAML", yamlErr)
		}
	}

	normalized := normalizeDynamicValue(root)
	document, ok := normalized.(map[string]any)
	if !ok {
		return nil, validationError("OpenAPI document root must be an object", nil)
	}
	return document, nil
}

func (g *HTTPResourceServerGateway) applyOpenAPIFallback(
	ctx context.Context,
	requestPath string,
	operation metadata.Operation,
	spec *metadata.OperationSpec,
	explicitMethod bool,
	explicitAccept bool,
	explicitContentType bool,
) error {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return err
	}

	_, pathItem, ok := findOpenAPIPathItem(document, requestPath)
	if !ok {
		return nil
	}

	if !explicitMethod && strings.TrimSpace(spec.Method) == "" {
		spec.Method = inferMethodFromPathItem(pathItem, operation)
	}

	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		return nil
	}

	operationItem, found := openAPIPathMethod(pathItem, method)
	if !found {
		return validationError(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, method), nil)
	}

	if !explicitAccept && strings.TrimSpace(spec.Accept) == "" {
		if accept := inferAcceptContentType(operationItem); accept != "" {
			spec.Accept = accept
		}
	}
	if !explicitContentType && strings.TrimSpace(spec.ContentType) == "" {
		if contentType := inferRequestContentType(operationItem); contentType != "" {
			spec.ContentType = contentType
		}
	}

	return nil
}

func (g *HTTPResourceServerGateway) validateOpenAPIMethodSupport(ctx context.Context, requestPath string, method string) error {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return err
	}

	_, pathItem, ok := findOpenAPIPathItem(document, requestPath)
	if !ok {
		return nil
	}

	if _, found := openAPIPathMethod(pathItem, method); !found {
		return validationError(fmt.Sprintf("OpenAPI path %q does not support method %s", requestPath, strings.ToUpper(method)), nil)
	}
	return nil
}

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
