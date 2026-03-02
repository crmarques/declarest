package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPManagedServerClient) Request(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (resource.Value, error) {
	resolvedMethod := strings.ToUpper(strings.TrimSpace(method))
	if resolvedMethod == "" {
		return nil, faults.NewValidationError("request method is required", nil)
	}

	resolvedPath := normalizeRequestPath(endpointPath)
	if resolvedPath == "" {
		return nil, faults.NewValidationError("request path is required", nil)
	}

	spec := metadata.OperationSpec{
		Method: resolvedMethod,
		Path:   resolvedPath,
		Accept: defaultMediaType,
		Body:   body,
	}
	if body != nil {
		spec.ContentType = defaultMediaType
	}

	_, hasOperation := requestMethodOperation(resolvedMethod)
	validationResource := resource.Resource{}
	validationMd := metadata.ResourceMetadata{}
	if _, resourceInput, validateSpec, ok := metadata.RequestOperationValidation(ctx); ok {
		hasOperation = true
		spec.Validate = validateSpec
		validationResource = resource.Resource{
			LogicalPath:    resourceInput.LogicalPath,
			CollectionPath: resourceInput.CollectionPath,
			LocalAlias:     resourceInput.LocalAlias,
			RemoteID:       resourceInput.RemoteID,
			Payload:        resourceInput.Payload,
		}
		validationMd = resourceInput.Metadata
	}
	if hasOperation && spec.Validate != nil {
		if err := g.validateOperationPayload(ctx, validationResource, validationMd, spec); err != nil {
			return nil, err
		}
	}

	responseBody, _, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return decodeRequestResponse(responseBody)
}

func (g *HTTPManagedServerClient) execute(ctx context.Context, spec metadata.OperationSpec) ([]byte, http.Header, error) {
	request, err := g.newRequest(ctx, spec)
	if err != nil {
		return nil, nil, err
	}

	response, err := g.doRequest(ctx, "resource", request)
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

func (g *HTTPManagedServerClient) newRequest(ctx context.Context, spec metadata.OperationSpec) (*http.Request, error) {
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

func (g *HTTPManagedServerClient) resolveRequestURL(requestPath string, query map[string]string) (string, error) {
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
