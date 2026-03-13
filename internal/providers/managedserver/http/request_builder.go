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
	if strings.TrimSpace(spec.Accept) == "" {
		spec.Accept, err = g.defaultResourceMediaType(bodyDescriptor)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	}

	if operationRequiresBody(operation) {
		if err := g.validateResourceMutationPayload(operation, resolvedResource, md, bodyDescriptor); err != nil {
			return metadata.OperationSpec{}, err
		}
		if strings.TrimSpace(spec.ContentType) == "" {
			spec.ContentType, err = g.defaultResourceMediaType(bodyDescriptor)
			if err != nil {
				return metadata.OperationSpec{}, err
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
