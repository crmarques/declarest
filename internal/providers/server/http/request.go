package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPResourceServerGateway) BuildRequestFromMetadata(ctx context.Context, resourceInfo resource.Resource, operation metadata.Operation) (metadata.OperationSpec, error) {
	spec, explicitMethod, explicitAccept, explicitContentType := operationSpecFromMetadata(resourceInfo.Metadata, operation)

	if strings.TrimSpace(spec.Path) == "" {
		spec.Path = fallbackOperationPath(resourceInfo, operation)
	}
	spec.Path = normalizeRequestPath(spec.Path)
	if spec.Path == "" {
		return metadata.OperationSpec{}, validationError("resolved operation path is empty", nil)
	}

	spec.Query = cloneStringMap(spec.Query)
	spec.Headers = mergeHeaders(g.defaultHeaders, spec.Headers)

	if err := g.applyOpenAPIFallback(ctx, spec.Path, operation, &spec, explicitMethod, explicitAccept, explicitContentType); err != nil {
		return metadata.OperationSpec{}, err
	}

	if strings.TrimSpace(spec.Method) == "" {
		spec.Method = defaultOperationMethod(operation)
	}
	spec.Method = strings.ToUpper(strings.TrimSpace(spec.Method))
	if spec.Method == "" {
		return metadata.OperationSpec{}, validationError(fmt.Sprintf("operation %q has no HTTP method", operation), nil)
	}

	if strings.TrimSpace(spec.Accept) == "" {
		spec.Accept = defaultMediaType
	}

	if operationRequiresBody(operation) {
		if strings.TrimSpace(spec.ContentType) == "" {
			spec.ContentType = defaultMediaType
		}
		if spec.Body == nil {
			spec.Body = resourceInfo.Payload
		}
	}

	if err := g.validateOpenAPIMethodSupport(ctx, spec.Path, spec.Method); err != nil {
		return metadata.OperationSpec{}, err
	}

	return spec, nil
}

func (g *HTTPResourceServerGateway) execute(ctx context.Context, spec metadata.OperationSpec) ([]byte, http.Header, error) {
	request, err := g.newRequest(ctx, spec)
	if err != nil {
		return nil, nil, err
	}

	response, err := g.client.Do(request)
	if err != nil {
		return nil, nil, transportError("remote request failed", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, nil, transportError("failed to read remote response body", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, nil, classifyStatusError(response.StatusCode, body)
	}

	return body, response.Header.Clone(), nil
}

func (g *HTTPResourceServerGateway) newRequest(ctx context.Context, spec metadata.OperationSpec) (*http.Request, error) {
	targetURL, err := g.resolveRequestURL(spec.Path, spec.Query)
	if err != nil {
		return nil, err
	}

	requestBody, err := encodeRequestBody(spec.ContentType, spec.Body)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if len(requestBody) > 0 {
		bodyReader = bytes.NewReader(requestBody)
	}

	request, err := http.NewRequestWithContext(ctx, spec.Method, targetURL, bodyReader)
	if err != nil {
		return nil, internalError("failed to create remote request", err)
	}

	if strings.TrimSpace(spec.Accept) != "" {
		request.Header.Set("Accept", spec.Accept)
	}
	if len(requestBody) > 0 && strings.TrimSpace(spec.ContentType) != "" {
		request.Header.Set("Content-Type", spec.ContentType)
	}

	if len(spec.Headers) > 0 {
		keys := make([]string, 0, len(spec.Headers))
		for key := range spec.Headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			request.Header.Set(key, spec.Headers[key])
		}
	}

	if err := g.applyAuth(ctx, request); err != nil {
		return nil, err
	}

	return request, nil
}

func (g *HTTPResourceServerGateway) resolveRequestURL(requestPath string, query map[string]string) (string, error) {
	if parsed, err := url.Parse(requestPath); err == nil && parsed.Scheme != "" {
		return "", validationError("operation path must be relative to managed-server.http.base-url", nil)
	}

	target := *g.baseURL
	target.Path = joinBaseAndRequestPath(g.baseURL.Path, requestPath)

	values := target.Query()
	if len(query) > 0 {
		keys := make([]string, 0, len(query))
		for key := range query {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			values.Set(key, query[key])
		}
	}
	target.RawQuery = values.Encode()

	return target.String(), nil
}

func (g *HTTPResourceServerGateway) decodeListResponse(collectionPath string, md metadata.ResourceMetadata, body []byte) ([]resource.Resource, error) {
	payload, err := decodeJSONResponse(body)
	if err != nil {
		return nil, err
	}

	items, err := extractListItems(payload)
	if err != nil {
		return nil, err
	}

	normalizedCollectionPath, err := resource.NormalizeLogicalPath(collectionPath)
	if err != nil {
		return nil, err
	}

	seenAliases := make(map[string]struct{}, len(items))
	list := make([]resource.Resource, 0, len(items))

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, validationError("list payload entries must be JSON objects", nil)
		}

		normalizedPayload, err := resource.Normalize(itemMap)
		if err != nil {
			return nil, err
		}

		payloadMap, ok := normalizedPayload.(map[string]any)
		if !ok {
			return nil, validationError("list payload entry normalization failed", nil)
		}

		alias, remoteID, err := resolveAliasAndRemoteID(payloadMap, md)
		if err != nil {
			return nil, err
		}
		if _, exists := seenAliases[alias]; exists {
			return nil, conflictError(fmt.Sprintf("remote list contains duplicate alias %q", alias), nil)
		}
		seenAliases[alias] = struct{}{}

		logicalPath, err := buildLogicalPath(normalizedCollectionPath, alias)
		if err != nil {
			return nil, err
		}

		list = append(list, resource.Resource{
			LogicalPath:    logicalPath,
			CollectionPath: normalizedCollectionPath,
			LocalAlias:     alias,
			RemoteID:       remoteID,
			Metadata:       md,
			Payload:        payloadMap,
		})
	}

	sort.Slice(list, func(i int, j int) bool {
		return list[i].LogicalPath < list[j].LogicalPath
	})
	return list, nil
}

func operationSpecFromMetadata(md metadata.ResourceMetadata, operation metadata.Operation) (metadata.OperationSpec, bool, bool, bool) {
	var spec metadata.OperationSpec
	if md.Operations != nil {
		spec = md.Operations[string(operation)]
	}

	explicitMethod := strings.TrimSpace(spec.Method) != ""
	explicitAccept := strings.TrimSpace(spec.Accept) != ""
	explicitContentType := strings.TrimSpace(spec.ContentType) != ""

	spec.Query = cloneStringMap(spec.Query)
	spec.Headers = cloneStringMap(spec.Headers)
	return spec, explicitMethod, explicitAccept, explicitContentType
}

func fallbackOperationPath(resourceInfo resource.Resource, operation metadata.Operation) string {
	if strings.TrimSpace(resourceInfo.ResolvedRemotePath) != "" {
		return resourceInfo.ResolvedRemotePath
	}
	if operation == metadata.OperationList && strings.TrimSpace(resourceInfo.CollectionPath) != "" {
		return resourceInfo.CollectionPath
	}
	if strings.TrimSpace(resourceInfo.LogicalPath) != "" {
		return resourceInfo.LogicalPath
	}
	return resourceInfo.CollectionPath
}

func defaultOperationMethod(operation metadata.Operation) string {
	switch operation {
	case metadata.OperationCreate:
		return http.MethodPost
	case metadata.OperationUpdate:
		return http.MethodPut
	case metadata.OperationDelete:
		return http.MethodDelete
	case metadata.OperationGet, metadata.OperationList, metadata.OperationCompare:
		return http.MethodGet
	default:
		return http.MethodGet
	}
}

func operationRequiresBody(operation metadata.Operation) bool {
	return operation == metadata.OperationCreate || operation == metadata.OperationUpdate
}

func normalizeRequestPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if trimmed != "/" {
		trimmed = strings.TrimSuffix(trimmed, "/")
	}
	return trimmed
}

func mergeHeaders(defaultHeaders map[string]string, operationHeaders map[string]string) map[string]string {
	if len(defaultHeaders) == 0 && len(operationHeaders) == 0 {
		return nil
	}

	merged := make(map[string]string, len(defaultHeaders)+len(operationHeaders))
	for key, value := range defaultHeaders {
		merged[key] = value
	}
	for key, value := range operationHeaders {
		merged[key] = value
	}
	return merged
}

func joinBaseAndRequestPath(basePath string, requestPath string) string {
	normalizedBase := normalizeRequestPath(basePath)
	if normalizedBase == "" {
		normalizedBase = "/"
	}

	normalizedRequest := normalizeRequestPath(requestPath)
	if normalizedRequest == "" || normalizedRequest == "/" {
		return normalizedBase
	}

	joined := path.Join(normalizedBase, strings.TrimPrefix(normalizedRequest, "/"))
	if !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	return joined
}

func encodeRequestBody(contentType string, body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	if strings.Contains(strings.ToLower(contentType), "json") || strings.TrimSpace(contentType) == "" {
		normalized, err := resource.Normalize(body)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return nil, validationError("failed to encode JSON request body", err)
		}
		return encoded, nil
	}

	switch typed := body.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, validationError("failed to encode request body", err)
		}
		return encoded, nil
	}
}

func decodeJSONResponse(body []byte) (resource.Value, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, validationError("response body is not valid JSON", err)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func classifyStatusError(statusCode int, body []byte) error {
	message := fmt.Sprintf("remote request failed with status %d: %s", statusCode, summarizeBody(body))

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return authError(message, nil)
	case http.StatusNotFound:
		return notFoundError(message, nil)
	case http.StatusConflict:
		return conflictError(message, nil)
	}

	if statusCode >= 400 && statusCode < 500 {
		return validationError(message, nil)
	}
	return transportError(message, nil)
}

func summarizeBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "<empty>"
	}
	if len(trimmed) > 512 {
		return trimmed[:512] + "..."
	}
	return trimmed
}

func extractListItems(payload any) ([]any, error) {
	switch typed := payload.(type) {
	case []any:
		return typed, nil
	case map[string]any:
		items, ok := typed["items"]
		if !ok {
			return nil, validationError("list response object must include an \"items\" array", nil)
		}
		values, ok := items.([]any)
		if !ok {
			return nil, validationError("list response \"items\" must be an array", nil)
		}
		return values, nil
	default:
		return nil, validationError("list response must be an array or an object with an \"items\" array", nil)
	}
}

func resolveAliasAndRemoteID(payload map[string]any, md metadata.ResourceMetadata) (string, string, error) {
	var alias string
	if strings.TrimSpace(md.AliasFromAttribute) != "" {
		alias, _ = lookupScalarAttribute(payload, md.AliasFromAttribute)
	}
	if alias == "" && strings.TrimSpace(md.IDFromAttribute) != "" {
		alias, _ = lookupScalarAttribute(payload, md.IDFromAttribute)
	}
	if alias == "" {
		return "", "", validationError("list item alias could not be resolved from metadata attributes", nil)
	}

	remoteID := alias
	if strings.TrimSpace(md.IDFromAttribute) != "" {
		if value, ok := lookupScalarAttribute(payload, md.IDFromAttribute); ok && value != "" {
			remoteID = value
		}
	}
	return alias, remoteID, nil
}

func lookupScalarAttribute(payload map[string]any, attribute string) (string, bool) {
	current := any(payload)
	for _, segment := range strings.Split(attribute, ".") {
		mapValue, ok := current.(map[string]any)
		if !ok {
			return "", false
		}

		next, exists := mapValue[segment]
		if !exists {
			return "", false
		}
		current = next
	}

	return scalarString(current)
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, typed != ""
	case json.Number:
		return typed.String(), true
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

func buildLogicalPath(collectionPath string, alias string) (string, error) {
	joined := path.Join(collectionPath, alias)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}

	normalized, err := resource.NormalizeLogicalPath(joined)
	if err != nil {
		return "", err
	}
	return normalized, nil
}
