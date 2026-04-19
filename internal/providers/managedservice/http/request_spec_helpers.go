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
	"context"
	"maps"
	"net/http"
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	managedservice "github.com/crmarques/declarest/managedservice"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func resolveOperationSpecTemplates(
	ctx context.Context,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
	spec metadata.OperationSpec,
	resolvedResource resource.Resource,
	descriptor resource.PayloadDescriptor,
) (metadata.OperationSpec, error) {
	templateScope, err := templatescope.BuildResourceScope(resolvedResource, md)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	metadata.ApplyPayloadTemplateScope(templateScope, md, resolvedResource.Payload, descriptor)

	templateMetadata := metadata.ResourceMetadata{
		RemoteCollectionPath: md.RemoteCollectionPath,
		Format:               md.Format,
		Operations: map[string]metadata.OperationSpec{
			string(operation): spec,
		},
		Transforms: metadata.CloneTransformSteps(md.Transforms),
	}

	rendered, err := metadata.ResolveOperationSpecWithScope(ctx, templateMetadata, operation, templateScope)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	return rendered, nil
}

func (g *Client) metadataPayloadDescriptor(md metadata.ResourceMetadata) resource.PayloadDescriptor {
	format := metadata.NormalizeResourceFormat(md.Format)
	if format == "" || metadata.ResourceFormatAllowsMixedItems(format) {
		return resource.PayloadDescriptor{}
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
		PayloadType: format,
	})
}

func (g *Client) defaultResourceMediaType(descriptor resource.PayloadDescriptor) (string, error) {
	if !resource.IsPayloadDescriptorExplicit(descriptor) {
		return "", faults.Invalid("payload descriptor is not concrete", nil)
	}
	mediaType := resource.NormalizePayloadDescriptor(descriptor).MediaType
	if strings.TrimSpace(mediaType) == "" {
		return "", faults.Invalid("invalid payload media type", nil)
	}
	return mediaType, nil
}

func (g *Client) requestFallbackDescriptor(
	ctx context.Context,
	requestSpec managedservice.RequestSpec,
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
	return metadata.InferPayloadDescriptor(spec.Body)
}

func (g *Client) requestBodyDescriptor(
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
) resource.PayloadDescriptor {
	return metadata.ResolveTemplatePayloadDescriptor(
		md,
		resolvedResource.Payload,
		resolvedResource.PayloadDescriptor,
	)
}

func (g *Client) requestAcceptDescriptor(
	operation metadata.Operation,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	bodyDescriptor resource.PayloadDescriptor,
) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(resolvedResource.PayloadDescriptor) {
		return resource.NormalizePayloadDescriptor(resolvedResource.PayloadDescriptor)
	}

	if descriptor := g.metadataPayloadDescriptor(md); resource.IsPayloadDescriptorExplicit(descriptor) {
		return descriptor
	}

	if operationRequiresBody(operation) && resource.IsPayloadDescriptorExplicit(bodyDescriptor) {
		return resource.NormalizePayloadDescriptor(bodyDescriptor)
	}

	return resource.PayloadDescriptor{}
}

func (g *Client) genericRequestBodyDescriptor(requestSpec managedservice.RequestSpec) resource.PayloadDescriptor {
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
		return metadata.InferPayloadDescriptor(requestSpec.Body)
	}
}

func (g *Client) payloadTemplateScopeDescriptor(
	md metadata.ResourceMetadata,
	resolvedResource resource.Resource,
) resource.PayloadDescriptor {
	return metadata.ResolveTemplatePayloadDescriptor(
		md,
		resolvedResource.Payload,
		resolvedResource.PayloadDescriptor,
	)
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
	normalizedBase := managedservice.NormalizeRequestPath(basePath)
	if normalizedBase == "" {
		normalizedBase = "/"
	}

	normalizedRequest := managedservice.NormalizeRequestPath(requestPath)
	if normalizedRequest == "" || normalizedRequest == "/" {
		return normalizedBase
	}

	joined := path.Join(normalizedBase, strings.TrimPrefix(normalizedRequest, "/"))
	if !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	return joined
}
