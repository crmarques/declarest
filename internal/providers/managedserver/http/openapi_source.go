package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPManagedServerClient) GetOpenAPISpec(ctx context.Context) (resource.Content, error) {
	doc, err := g.openAPIDocument(ctx)
	if err != nil {
		return resource.Content{}, err
	}
	return resource.Content{
		Value:      cloneValue(doc),
		Descriptor: g.openAPISourceDescriptor(),
	}, nil
}

func validateOpenAPISource(source string) error {
	value := strings.TrimSpace(source)
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return faults.NewValidationError("managed-server.http.openapi is invalid", err)
	}

	if parsed.Scheme == "" {
		return nil
	}
	if parsed.Scheme != "https" {
		return faults.NewValidationError("managed-server.http.openapi must use https when configured as URL", nil)
	}
	if parsed.Host == "" {
		return faults.NewValidationError("managed-server.http.openapi URL host is required", nil)
	}
	return nil
}

func (g *HTTPManagedServerClient) openAPIDocument(ctx context.Context) (map[string]any, error) {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil, faults.NewValidationError("managed-server.http.openapi is not configured", nil)
	}

	g.openapiMu.Lock()
	defer g.openapiMu.Unlock()

	if g.openapiLoaded {
		return g.openapiDoc, g.openapiErr
	}

	document, err := g.loadOpenAPIDocument(ctx)
	if err != nil {
		return nil, err
	}

	g.openapiDoc = document
	g.openapiLoaded = true
	return g.openapiDoc, nil
}

func (g *HTTPManagedServerClient) loadOpenAPIDocument(ctx context.Context) (map[string]any, error) {
	source := strings.TrimSpace(g.openAPISource)
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, faults.NewValidationError("managed-server.http.openapi is invalid", err)
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
		if sameURLOffsetOrigin(g.baseURL, parsed) {
			if err := g.applyAuth(ctx, request); err != nil {
				return nil, err
			}
		}

		response, err := g.doRequest(ctx, "openapi", request)
		if err != nil {
			return nil, transportError("failed to fetch OpenAPI document", err)
		}
		defer func() {
			_ = response.Body.Close()
		}()

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
		return nil, faults.NewValidationError("managed-server.http.openapi must be a local file path or https URL", nil)
	}

	var root any
	if jsonErr := json.Unmarshal(content, &root); jsonErr != nil {
		if yamlErr := yaml.Unmarshal(content, &root); yamlErr != nil {
			return nil, faults.NewValidationError("OpenAPI document must be valid JSON or YAML", yamlErr)
		}
	}

	normalized := normalizeDynamicValue(root)
	document, ok := normalized.(map[string]any)
	if !ok {
		return nil, faults.NewValidationError("OpenAPI document root must be an object", nil)
	}
	document = normalizeOpenAPIDocument(document)
	return document, nil
}

func (g *HTTPManagedServerClient) openAPISourceDescriptor() resource.PayloadDescriptor {
	source := strings.TrimSpace(g.openAPISource)
	if descriptor, ok := resource.PayloadDescriptorForFileName(source); ok {
		return descriptor
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		PayloadType: resource.PayloadTypeJSON,
	})
}

func sameURLOffsetOrigin(a *url.URL, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	if !strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}
	return effectiveURLPort(a) == effectiveURLPort(b)
}

func effectiveURLPort(value *url.URL) string {
	if value == nil {
		return ""
	}
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
