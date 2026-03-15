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
	"fmt"
	"maps"
	"strings"

	"github.com/crmarques/declarest/faults"
	managedserver "github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) BuildRequestFromMetadata(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata, operation metadata.Operation) (metadata.OperationSpec, error) {
	spec, explicitPath, explicitMethod, explicitAccept, explicitContentType := operationSpecFromMetadata(md, operation)

	var err error
	if g.metadataRenderer != nil {
		spec, err = g.metadataRenderer.RenderOperationSpecForResource(ctx, metadata.ResourceOperationSpecInput{
			LogicalPath:       resolvedResource.LogicalPath,
			CollectionPath:    resolvedResource.CollectionPath,
			LocalAlias:        resolvedResource.LocalAlias,
			RemoteID:          resolvedResource.RemoteID,
			PayloadDescriptor: resolvedResource.PayloadDescriptor,
			Metadata:          md,
			Payload:           resolvedResource.Payload,
		}, operation)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	} else {
		spec, err = resolveOperationSpecTemplates(
			ctx,
			md,
			operation,
			spec,
			resolvedResource,
			g.payloadTemplateScopeDescriptor(md, resolvedResource),
		)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	}
	if !explicitPath && strings.TrimSpace(resolvedResource.ResolvedRemotePath) != "" {
		spec.Path = resolvedResource.ResolvedRemotePath
	}
	if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
		spec.Method = overrideMethod
		explicitMethod = true
	}
	spec.Path = managedserver.NormalizeRequestPath(spec.Path)
	if spec.Path == "" {
		return metadata.OperationSpec{}, faults.NewValidationError("resolved operation path is empty", nil)
	}

	spec.Query = maps.Clone(spec.Query)
	spec.Headers = mergeHeaders(g.defaultHeaders, spec.Headers)

	if err := g.applyOpenAPIFallback(ctx, spec.Path, operation, &spec, explicitMethod, explicitAccept, explicitContentType); err != nil {
		return metadata.OperationSpec{}, err
	}

	if strings.TrimSpace(spec.Method) == "" {
		spec.Method = defaultOperationMethod(operation)
	}
	spec.Method = strings.ToUpper(strings.TrimSpace(spec.Method))
	if spec.Method == "" {
		return metadata.OperationSpec{}, faults.NewValidationError(fmt.Sprintf("operation %q has no HTTP method", operation), nil)
	}

	bodyDescriptor := g.requestBodyDescriptor(resolvedResource, md)
	acceptDescriptor := g.requestAcceptDescriptor(operation, resolvedResource, md, bodyDescriptor)
	if strings.TrimSpace(spec.Accept) == "" {
		if mediaType, mediaErr := g.defaultResourceMediaType(acceptDescriptor); mediaErr == nil {
			spec.Accept = mediaType
		}
	}

	if operationRequiresBody(operation) {
		if err := g.validateResourceMutationPayload(operation, resolvedResource, md, bodyDescriptor); err != nil {
			return metadata.OperationSpec{}, err
		}
		if strings.TrimSpace(spec.ContentType) == "" {
			if mediaType, mediaErr := g.defaultResourceMediaType(bodyDescriptor); mediaErr == nil {
				spec.ContentType = mediaType
			}
		}
		if spec.Body == nil {
			spec.Body = resource.Content{
				Value:      resolvedResource.Payload,
				Descriptor: bodyDescriptor,
			}
		}
		transformedBody := unwrapContentValue(spec.Body)
		if resource.IsStructuredPayloadType(bodyDescriptor.PayloadType) {
			transformedBody, err = g.applyOperationPayloadTransforms(ctx, spec.Body, spec)
			if err != nil {
				return metadata.OperationSpec{}, err
			}
		}
		spec.Body = resource.Content{
			Value:      transformedBody,
			Descriptor: bodyDescriptor,
		}
	}

	if err := g.validateOpenAPIMethodSupport(ctx, spec.Path, spec.Method); err != nil {
		return metadata.OperationSpec{}, err
	}
	if err := g.validateOperationPayload(ctx, resolvedResource, md, spec); err != nil {
		return metadata.OperationSpec{}, err
	}

	return spec, nil
}
