package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) Request(
	ctx context.Context,
	requestSpec managedserverdomain.RequestSpec,
) (resource.Content, error) {
	resolvedMethod := strings.ToUpper(strings.TrimSpace(requestSpec.Method))
	if resolvedMethod == "" {
		return resource.Content{}, faults.NewValidationError("request method is required", nil)
	}

	resolvedPath := normalizeRequestPath(requestSpec.Path)
	if resolvedPath == "" {
		return resource.Content{}, faults.NewValidationError("request path is required", nil)
	}

	bodyDescriptor := g.genericRequestBodyDescriptor(requestSpec)
	spec := metadata.OperationSpec{
		Method:      resolvedMethod,
		Path:        resolvedPath,
		Query:       maps.Clone(requestSpec.Query),
		Headers:     maps.Clone(requestSpec.Headers),
		Accept:      requestSpec.Accept,
		ContentType: requestSpec.ContentType,
		Body: resource.Content{
			Value:      requestSpec.Body.Value,
			Descriptor: bodyDescriptor,
		},
	}
	if strings.TrimSpace(spec.Accept) == "" {
		spec.Accept = bodyDescriptor.MediaType
	}
	if requestSpec.Body.Value != nil && strings.TrimSpace(spec.ContentType) == "" {
		spec.ContentType = bodyDescriptor.MediaType
	}

	operation, hasOperation := requestMethodOperation(resolvedMethod)
	validationResource := resource.Resource{}
	validationMd := metadata.ResourceMetadata{}
	if ctxOperation, resourceInput, validateSpec, ok := metadata.RequestOperationValidation(ctx); ok {
		hasOperation = true
		operation = ctxOperation
		spec.Validate = validateSpec
		validationResource = resource.Resource{
			LogicalPath:       resourceInput.LogicalPath,
			CollectionPath:    resourceInput.CollectionPath,
			LocalAlias:        resourceInput.LocalAlias,
			RemoteID:          resourceInput.RemoteID,
			Payload:           resourceInput.Payload,
			PayloadDescriptor: bodyDescriptor,
		}
		validationMd = resourceInput.Metadata
	}
	if validationResource.Payload == nil && requestSpec.Body.Value != nil {
		validationResource.Payload = requestSpec.Body.Value
		validationResource.PayloadDescriptor = bodyDescriptor
	}
	if hasOperation && operationRequiresBody(operation) {
		if err := g.validateResourceMutationPayload(validationResource, validationMd, bodyDescriptor); err != nil {
			return resource.Content{}, err
		}
	}
	if hasOperation && spec.Validate != nil {
		if err := g.validateOperationPayload(ctx, validationResource, validationMd, spec); err != nil {
			return resource.Content{}, err
		}
	}

	responseBody, responseHeaders, err := g.execute(ctx, spec)
	if err != nil {
		return resource.Content{}, err
	}

	return decodeResponseBody(responseBody, responseHeaders, g.requestFallbackDescriptor(ctx, requestSpec, spec))
}

func (g *Client) execute(ctx context.Context, spec metadata.OperationSpec) ([]byte, http.Header, error) {
	request, err := g.newRequest(ctx, spec)
	if err != nil {
		return nil, nil, err
	}

	// Level 3: log request body
	debugRequestBody(ctx, request)

	response, err := g.doRequest(ctx, "resource", request)
	if err != nil {
		return nil, nil, transportError("remote request failed", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, nil, transportError("failed to read remote response body", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		debugErrorResponse(ctx, spec.Method, spec.Path, response.StatusCode, body)
		return nil, nil, classifyStatusError(response.StatusCode, body)
	}

	// Level 3: log successful response body
	debugctx.Printf(ctx, "http response body length=%d content=%s", len(body), summarizeBodyForLevel(body, 3))

	return body, response.Header.Clone(), nil
}

// debugRequestBody logs the request body at trace level (3).
func debugRequestBody(ctx context.Context, request *http.Request) {
	if debugctx.Level(ctx) < 3 || request == nil || request.Body == nil {
		return
	}
	if request.GetBody == nil {
		debugctx.Printf(ctx, "http request body <not replayable>")
		return
	}
	bodyReader, err := request.GetBody()
	if err != nil {
		debugctx.Printf(ctx, "http request body <read error: %v>", err)
		return
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(bodyReader, 1<<20))
	if err != nil {
		debugctx.Printf(ctx, "http request body <read error: %v>", err)
		return
	}
	if len(bodyBytes) == 0 {
		return
	}
	debugctx.Printf(ctx, "http request body length=%d content=%s", len(bodyBytes), string(bodyBytes))
}

// debugErrorResponse logs the full error response at the appropriate verbosity level.
// Level 1: enriched error with full response body, method, and path.
// Level 2+: same but already covered by doRequest logging.
func debugErrorResponse(ctx context.Context, method string, path string, statusCode int, body []byte) {
	level := debugctx.Level(ctx)
	if level < 1 {
		return
	}

	debugctx.Infof(
		ctx,
		"managed-server error method=%s path=%q status=%d response_body=%s",
		method,
		path,
		statusCode,
		summarizeBodyForLevel(body, level),
	)
}

// summarizeBodyForLevel returns the response body truncated according to verbosity level.
//
//	Level 1: up to 1024 characters
//	Level 2: up to 4096 characters
//	Level 3: full body (no truncation)
func summarizeBodyForLevel(body []byte, level int) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "<empty>"
	}

	limit := 0
	switch {
	case level >= 3:
		return trimmed
	case level == 2:
		limit = 4096
	default:
		limit = 1024
	}

	if len(trimmed) <= limit {
		return trimmed
	}
	return fmt.Sprintf("%s... (%d bytes truncated)", trimmed[:limit], len(trimmed)-limit)
}

func (g *Client) newRequest(ctx context.Context, spec metadata.OperationSpec) (*http.Request, error) {
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

func (g *Client) resolveRequestURL(requestPath string, query map[string]string) (string, error) {
	if parsed, err := url.Parse(requestPath); err == nil && parsed.Scheme != "" {
		return "", faults.NewValidationError("operation path must be relative to managed-server.http.base-url", nil)
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
