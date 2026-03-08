package http

import (
	"context"
	"maps"
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
	descriptor resource.PayloadDescriptor,
) (metadata.OperationSpec, error) {
	templateScope, err := templatescope.BuildResourceScope(resourceInfo, md)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	applyPayloadTemplateScope(templateScope, md, descriptor)

	templateMetadata := metadata.ResourceMetadata{
		CollectionPath: md.CollectionPath,
		PayloadType:    md.PayloadType,
		Operations: map[string]metadata.OperationSpec{
			string(operation): spec,
		},
		PayloadMutation: metadata.CloneResourceMetadata(metadata.ResourceMetadata{PayloadMutation: md.PayloadMutation}).PayloadMutation,
	}

	rendered, err := metadata.ResolveOperationSpecWithScope(ctx, templateMetadata, operation, templateScope)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	return rendered, nil
}

func (g *Client) metadataPayloadDescriptor(md metadata.ResourceMetadata) resource.PayloadDescriptor {
	if strings.TrimSpace(md.PayloadType) == "" {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		PayloadType: md.PayloadType,
	})
}

func (g *Client) defaultResourceMediaType(descriptor resource.PayloadDescriptor) (string, error) {
	mediaType := resource.NormalizePayloadDescriptor(descriptor).MediaType
	if strings.TrimSpace(mediaType) == "" {
		return "", faults.NewValidationError("invalid payload media type", nil)
	}
	return mediaType, nil
}

func (g *Client) requestFallbackDescriptor(
	ctx context.Context,
	requestSpec managedserver.RequestSpec,
	spec metadata.OperationSpec,
) resource.PayloadDescriptor {
	if _, resourceInput, _, ok := metadata.RequestOperationValidation(ctx); ok {
		return g.metadataPayloadDescriptor(resourceInput.Metadata)
	}
	if descriptor, ok := resource.PayloadDescriptorForContentType(requestSpec.Accept); ok {
		return descriptor
	}
	if descriptor, ok := resource.PayloadDescriptorForContentType(spec.Accept); ok {
		return descriptor
	}
	if descriptor, ok := resource.PayloadDescriptorForContentType(requestSpec.ContentType); ok {
		return descriptor
	}
	if descriptor, ok := resource.PayloadDescriptorForContentType(spec.ContentType); ok {
		return descriptor
	}
	if resource.IsPayloadDescriptorExplicit(requestSpec.Body.Descriptor) {
		return resource.NormalizePayloadDescriptor(requestSpec.Body.Descriptor)
	}
	if resource.IsBinaryValue(requestSpec.Body.Value) || resource.IsBinaryValue(spec.Body) {
		return resource.DefaultOctetStreamDescriptor()
	}
	if typed, ok := spec.Body.(resource.Content); ok && resource.IsPayloadDescriptorExplicit(typed.Descriptor) {
		return resource.NormalizePayloadDescriptor(typed.Descriptor)
	}
	return payloadDescriptorFromValue(spec.Body)
}

func (g *Client) requestBodyDescriptor(
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
) resource.PayloadDescriptor {
	switch {
	case resource.IsPayloadDescriptorExplicit(resourceInfo.PayloadDescriptor):
		return resource.NormalizePayloadDescriptor(resourceInfo.PayloadDescriptor)
	case strings.TrimSpace(md.PayloadType) != "":
		return g.metadataPayloadDescriptor(md)
	default:
		return payloadDescriptorFromValue(resourceInfo.Payload)
	}
}

func (g *Client) genericRequestBodyDescriptor(requestSpec managedserver.RequestSpec) resource.PayloadDescriptor {
	switch {
	case resource.IsPayloadDescriptorExplicit(requestSpec.Body.Descriptor):
		return resource.NormalizePayloadDescriptor(requestSpec.Body.Descriptor)
	case strings.TrimSpace(requestSpec.ContentType) != "":
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			MediaType: requestSpec.ContentType,
		})
	case resource.IsBinaryValue(requestSpec.Body.Value):
		return resource.DefaultOctetStreamDescriptor()
	default:
		return payloadDescriptorFromValue(requestSpec.Body.Value)
	}
}

func (g *Client) payloadTemplateScopeDescriptor(
	md metadata.ResourceMetadata,
	resourceInfo resource.Resource,
) resource.PayloadDescriptor {
	switch {
	case resource.IsPayloadDescriptorExplicit(resourceInfo.PayloadDescriptor):
		return resource.NormalizePayloadDescriptor(resourceInfo.PayloadDescriptor)
	case strings.TrimSpace(md.PayloadType) != "":
		return g.metadataPayloadDescriptor(md)
	default:
		return payloadDescriptorFromValue(resourceInfo.Payload)
	}
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

	spec.Query = maps.Clone(spec.Query)
	spec.Headers = maps.Clone(spec.Headers)
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

func applyPayloadTemplateScope(scope map[string]any, md metadata.ResourceMetadata, descriptor resource.PayloadDescriptor) {
	if scope == nil {
		return
	}

	activeDescriptor := descriptor
	if !resource.IsPayloadDescriptorExplicit(activeDescriptor) {
		if strings.TrimSpace(md.PayloadType) != "" {
			activeDescriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: md.PayloadType})
		} else {
			activeDescriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
		}
	}
	scope["payloadType"] = activeDescriptor.PayloadType
	scope["payloadMediaType"] = activeDescriptor.MediaType
	scope["payloadExtension"] = activeDescriptor.Extension
}

func payloadDescriptorFromValue(value any) resource.PayloadDescriptor {
	value = unwrapContentValue(value)

	switch typed := value.(type) {
	case nil:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	case resource.BinaryValue:
		return resource.DefaultOctetStreamDescriptor()
	case *resource.BinaryValue:
		if typed != nil {
			return resource.DefaultOctetStreamDescriptor()
		}
	case string:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
	switch normalized.(type) {
	case string:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	case resource.BinaryValue:
		return resource.DefaultOctetStreamDescriptor()
	default:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
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
