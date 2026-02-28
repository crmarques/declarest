package orchestrator

import (
	"context"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

func (r *DefaultOrchestrator) Request(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	resolvedPath, requestCtx, err := r.resolveRequestEndpointPath(ctx, method, endpointPath, body)
	if err != nil {
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator request method=%q path=%q resolved_path=%q has_body=%t",
		method,
		endpointPath,
		resolvedPath,
		body != nil,
	)

	value, err := serverManager.Request(requestCtx, method, resolvedPath, body)
	if err != nil {
		if fallbackValue, handled, fallbackErr := r.retryRequestResolvedMutationWithLiteralPath(
			requestCtx,
			serverManager,
			method,
			endpointPath,
			resolvedPath,
			body,
			err,
		); handled {
			if fallbackErr != nil {
				debugctx.Printf(
					ctx,
					"orchestrator request literal fallback failed method=%q path=%q resolved_path=%q error=%v",
					method,
					endpointPath,
					resolvedPath,
					fallbackErr,
				)
				return nil, fallbackErr
			}
			debugctx.Printf(
				ctx,
				"orchestrator request literal fallback succeeded method=%q path=%q resolved_path=%q value_type=%T",
				method,
				endpointPath,
				resolvedPath,
				fallbackValue,
			)
			return fallbackValue, nil
		}

		if isTypedCategory(err, faults.NotFoundError) {
			fallbackValue, handled, fallbackErr := r.retryRequestNotFoundWithMetadata(
				requestCtx,
				serverManager,
				method,
				endpointPath,
				body,
			)
			if handled {
				if fallbackErr != nil {
					debugctx.Printf(
						ctx,
						"orchestrator request fallback failed method=%q path=%q resolved_path=%q error=%v",
						method,
						endpointPath,
						resolvedPath,
						fallbackErr,
					)
					return nil, fallbackErr
				}
				debugctx.Printf(
					ctx,
					"orchestrator request fallback succeeded method=%q path=%q resolved_path=%q value_type=%T",
					method,
					endpointPath,
					resolvedPath,
					fallbackValue,
				)
				return fallbackValue, nil
			}
		}
		debugctx.Printf(
			ctx,
			"orchestrator request failed method=%q path=%q resolved_path=%q error=%v",
			method,
			endpointPath,
			resolvedPath,
			err,
		)
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator request succeeded method=%q path=%q resolved_path=%q value_type=%T",
		method,
		endpointPath,
		resolvedPath,
		value,
	)
	return value, nil
}

func (r *DefaultOrchestrator) retryRequestResolvedMutationWithLiteralPath(
	ctx context.Context,
	serverManager server.ResourceServer,
	method string,
	endpointPath string,
	resolvedPath string,
	body resource.Value,
	requestErr error,
) (resource.Value, bool, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod != "PUT" && normalizedMethod != "PATCH" {
		return nil, false, nil
	}
	if !isTypedCategory(requestErr, faults.NotFoundError) {
		return nil, false, nil
	}
	if sameNormalizedRequestPath(endpointPath, resolvedPath) {
		return nil, false, nil
	}

	value, err := serverManager.Request(ctx, method, endpointPath, body)
	return value, true, err
}

func (r *DefaultOrchestrator) retryRequestNotFoundWithMetadata(
	ctx context.Context,
	serverManager server.ResourceServer,
	method string,
	endpointPath string,
	body resource.Value,
) (resource.Value, bool, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))

	switch normalizedMethod {
	case "GET":
		value, err := r.GetRemote(ctx, endpointPath)
		return value, true, err
	case "DELETE":
		resourceInfo, err := r.buildResourceInfoForRemoteRead(ctx, endpointPath)
		if err != nil {
			return nil, true, err
		}

		remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo)
		if err != nil {
			return nil, true, err
		}

		normalizedPayload, err := resource.Normalize(remoteValue)
		if err != nil {
			return nil, true, err
		}
		resourceInfo.Payload = normalizedPayload

		localAlias, remoteID, err := resolveResourceIdentity(
			resourceInfo.LogicalPath,
			resourceInfo.Metadata,
			normalizedPayload,
		)
		if err != nil {
			return nil, true, err
		}
		resourceInfo.LocalAlias = localAlias
		resourceInfo.RemoteID = remoteID

		spec, err := r.renderOperationSpec(ctx, resourceInfo, metadata.OperationDelete, resourceInfo.Payload)
		if err != nil {
			return nil, true, err
		}

		value, err := serverManager.Request(ctx, method, spec.Path, body)
		return value, true, err
	default:
		return nil, false, nil
	}
}

func (r *DefaultOrchestrator) resolveRequestEndpointPath(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (string, context.Context, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))

	operation, ok := requestMetadataOperation(normalizedMethod)
	if !ok {
		return endpointPath, ctx, nil
	}

	resourceInfo, err := r.buildResourceInfoForRemoteRead(ctx, endpointPath)
	if err != nil {
		return "", ctx, err
	}

	if normalizedMethod == "GET" && shouldResolveRequestGetAsList(resourceInfo) {
		operation = metadata.OperationList
	}

	// Collection-target operations should preserve the requested logical path as
	// the default collection path when metadata does not override it.
	if operation == metadata.OperationCreate || operation == metadata.OperationList {
		resourceInfo.CollectionPath = resourceInfo.LogicalPath
	}

	if body != nil {
		normalizedBody, normalizeErr := resource.Normalize(body)
		if normalizeErr != nil {
			return "", ctx, normalizeErr
		}
		resourceInfo.Payload = normalizedBody
	}

	spec, err := r.renderOperationSpec(ctx, resourceInfo, operation, resourceInfo.Payload)
	if err != nil {
		return "", ctx, err
	}

	ctxWithValidation := metadata.WithRequestOperationValidation(
		ctx,
		operation,
		metadata.ResourceOperationSpecInput{
			LogicalPath:    resourceInfo.LogicalPath,
			CollectionPath: resourceInfo.CollectionPath,
			LocalAlias:     resourceInfo.LocalAlias,
			RemoteID:       resourceInfo.RemoteID,
			Metadata:       resourceInfo.Metadata,
			Payload:        resourceInfo.Payload,
		},
		spec.Validate,
	)

	return spec.Path, ctxWithValidation, nil
}

func requestMetadataOperation(method string) (metadata.Operation, bool) {
	switch method {
	case "GET":
		return metadata.OperationGet, true
	case "POST":
		return metadata.OperationCreate, true
	case "PUT", "PATCH":
		return metadata.OperationUpdate, true
	case "DELETE":
		return metadata.OperationDelete, true
	default:
		return "", false
	}
}

func shouldResolveRequestGetAsList(resourceInfo resource.Resource) bool {
	md := resourceInfo.Metadata
	if strings.TrimSpace(md.CollectionPath) == "" {
		return false
	}

	if md.Operations != nil {
		if getSpec, ok := md.Operations[string(metadata.OperationGet)]; ok && strings.TrimSpace(getSpec.Path) != "" {
			return false
		}
	}

	templateDepth := len(splitLogicalPathSegments(md.CollectionPath))
	if templateDepth == 0 {
		return false
	}

	logicalDepth := len(splitLogicalPathSegments(resourceInfo.LogicalPath))
	if logicalDepth == 0 {
		return false
	}

	// Selector-depth logical paths (for example /.../user-registry) often map to
	// remote collection endpoints via metadata collectionPath overrides. For
	// request GET, prefer the list path in this case so the raw request targets
	// the correct collection endpoint (for example /components) before fallback.
	return logicalDepth <= templateDepth
}

func sameNormalizedRequestPath(first string, second string) bool {
	normalizedFirst, errFirst := resource.NormalizeLogicalPath(first)
	normalizedSecond, errSecond := resource.NormalizeLogicalPath(second)
	if errFirst == nil && errSecond == nil {
		return normalizedFirst == normalizedSecond
	}

	return strings.TrimSpace(first) == strings.TrimSpace(second)
}

func (r *DefaultOrchestrator) GetOpenAPISpec(ctx context.Context) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}
	return serverManager.GetOpenAPISpec(ctx)
}
