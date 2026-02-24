package http

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func resolveOperationSpecTemplates(
	ctx context.Context,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
	spec metadata.OperationSpec,
	resourceInfo resource.Resource,
	resourceFormat string,
) (metadata.OperationSpec, error) {
	templateScope, err := templatescope.BuildResourceScope(resourceInfo)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	templateScope["resourceFormat"] = metadata.NormalizeResourceFormat(resourceFormat)

	templateMetadata := metadata.ResourceMetadata{
		CollectionPath: md.CollectionPath,
		Operations: map[string]metadata.OperationSpec{
			string(operation): spec,
		},
		Filter:   cloneStringSlice(md.Filter),
		Suppress: cloneStringSlice(md.Suppress),
		JQ:       md.JQ,
	}

	rendered, err := metadata.ResolveOperationSpecWithScope(ctx, templateMetadata, operation, templateScope)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	return rendered, nil
}

func (g *HTTPResourceServerGateway) effectiveResourceFormat() string {
	if g == nil {
		return metadata.NormalizeResourceFormat("")
	}
	return metadata.NormalizeResourceFormat(g.resourceFormat)
}

func (g *HTTPResourceServerGateway) defaultResourceMediaType() (string, error) {
	mediaType, err := metadata.ResourceFormatMediaType(g.effectiveResourceFormat())
	if err != nil {
		return "", validationError("invalid repository resource format", err)
	}
	return mediaType, nil
}

func operationSpecFromMetadata(md metadata.ResourceMetadata, operation metadata.Operation) (metadata.OperationSpec, bool, bool, bool, bool) {
	var spec metadata.OperationSpec
	if md.Operations != nil {
		spec = md.Operations[string(operation)]
	}

	explicitPath := strings.TrimSpace(spec.Path) != ""
	explicitMethod := strings.TrimSpace(spec.Method) != ""
	explicitAccept := strings.TrimSpace(spec.Accept) != ""
	explicitContentType := strings.TrimSpace(spec.ContentType) != ""

	spec.Query = cloneStringMap(spec.Query)
	spec.Headers = cloneStringMap(spec.Headers)
	return spec, explicitPath, explicitMethod, explicitAccept, explicitContentType
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

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
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
