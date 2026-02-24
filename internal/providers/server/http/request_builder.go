package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPResourceServerGateway) BuildRequestFromMetadata(ctx context.Context, resourceInfo resource.Resource, operation metadata.Operation) (metadata.OperationSpec, error) {
	spec, explicitPath, explicitMethod, explicitAccept, explicitContentType := operationSpecFromMetadata(resourceInfo.Metadata, operation)

	var err error
	if g.metadataRenderer != nil {
		spec, err = g.metadataRenderer.RenderOperationSpecForResource(ctx, metadata.ResourceOperationSpecInput{
			LogicalPath:    resourceInfo.LogicalPath,
			CollectionPath: resourceInfo.CollectionPath,
			LocalAlias:     resourceInfo.LocalAlias,
			RemoteID:       resourceInfo.RemoteID,
			Metadata:       resourceInfo.Metadata,
			Payload:        resourceInfo.Payload,
		}, operation)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	} else {
		spec, err = resolveOperationSpecTemplates(
			ctx,
			resourceInfo.Metadata,
			operation,
			spec,
			resourceInfo,
			g.effectiveResourceFormat(),
		)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	}
	if !explicitPath && strings.TrimSpace(resourceInfo.ResolvedRemotePath) != "" {
		spec.Path = resourceInfo.ResolvedRemotePath
	}
	if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
		spec.Method = overrideMethod
		explicitMethod = true
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
		spec.Accept, err = g.defaultResourceMediaType()
		if err != nil {
			return metadata.OperationSpec{}, err
		}
	}

	if operationRequiresBody(operation) {
		if strings.TrimSpace(spec.ContentType) == "" {
			spec.ContentType, err = g.defaultResourceMediaType()
			if err != nil {
				return metadata.OperationSpec{}, err
			}
		}
		if spec.Body == nil {
			spec.Body = resourceInfo.Payload
		}
		transformedBody, err := g.applyOperationPayloadTransforms(ctx, spec.Body, spec)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
		spec.Body = transformedBody
	}

	if err := g.validateOpenAPIMethodSupport(ctx, spec.Path, spec.Method); err != nil {
		return metadata.OperationSpec{}, err
	}

	return spec, nil
}
