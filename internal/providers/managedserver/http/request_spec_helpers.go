package http

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	managedserver "github.com/crmarques/declarest/managedserver"
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
	templateScope, err := templatescope.BuildResourceScope(resourceInfo, md)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	applyPayloadTemplateScope(templateScope, md, resourceFormat)

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

func (g *HTTPManagedServerClient) effectiveResourceFormat() string {
	if g == nil {
		return metadata.NormalizeResourceFormat("")
	}
	return metadata.NormalizeResourceFormat(g.resourceFormat)
}

func (g *HTTPManagedServerClient) metadataPayloadType(md metadata.ResourceMetadata) string {
	payloadType, err := metadata.EffectivePayloadType(md, g.effectiveResourceFormat())
	if err != nil {
		return g.effectiveResourceFormat()
	}
	return payloadType
}

func (g *HTTPManagedServerClient) defaultResourceMediaType(payloadType string) (string, error) {
	mediaType, err := metadata.ResourceFormatMediaType(payloadType)
	if err != nil {
		return "", faults.NewValidationError("invalid repository resource format", err)
	}
	return mediaType, nil
}

func (g *HTTPManagedServerClient) requestFallbackPayloadType(
	ctx context.Context,
	requestSpec managedserver.RequestSpec,
	spec metadata.OperationSpec,
) string {
	if _, resourceInput, _, ok := metadata.RequestOperationValidation(ctx); ok {
		return g.metadataPayloadType(resourceInput.Metadata)
	}
	if payloadType, ok := resource.PayloadTypeForMediaType(requestSpec.Accept); ok {
		return payloadType
	}
	if payloadType, ok := resource.PayloadTypeForMediaType(spec.Accept); ok {
		return payloadType
	}
	if payloadType, ok := resource.PayloadTypeForMediaType(requestSpec.ContentType); ok {
		return payloadType
	}
	if payloadType, ok := resource.PayloadTypeForMediaType(spec.ContentType); ok {
		return payloadType
	}
	if resource.IsBinaryValue(spec.Body) {
		return resource.PayloadTypeOctetStream
	}
	return g.effectiveResourceFormat()
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

func requestMethodOperation(method string) (metadata.Operation, bool) {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet:
		return metadata.OperationGet, true
	case http.MethodPost:
		return metadata.OperationCreate, true
	case http.MethodPut, http.MethodPatch:
		return metadata.OperationUpdate, true
	case http.MethodDelete:
		return metadata.OperationDelete, true
	default:
		return "", false
	}
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

func applyPayloadTemplateScope(scope map[string]any, md metadata.ResourceMetadata, resourceFormat string) {
	if scope == nil {
		return
	}

	scope["resourceFormat"] = metadata.NormalizeResourceFormat(resourceFormat)

	payloadType, err := metadata.EffectivePayloadType(md, resourceFormat)
	if err != nil {
		payloadType = metadata.NormalizeResourceFormat(resourceFormat)
	}
	scope["payloadType"] = payloadType

	if mediaType, mediaErr := metadata.ResourceFormatMediaType(payloadType); mediaErr == nil {
		scope["payloadMediaType"] = mediaType
	}
	if extension, extensionErr := metadata.ResourceFormatExtension(payloadType); extensionErr == nil {
		scope["payloadExtension"] = extension
	}
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
