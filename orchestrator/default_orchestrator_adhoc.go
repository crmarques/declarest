package orchestrator

import (
	"context"
	"strings"

	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) AdHoc(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	resolvedPath, err := r.resolveAdHocEndpointPath(ctx, method, endpointPath, body)
	if err != nil {
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator ad-hoc request method=%q path=%q resolved_path=%q has_body=%t",
		method,
		endpointPath,
		resolvedPath,
		body != nil,
	)

	value, err := serverManager.AdHoc(ctx, method, resolvedPath, body)
	if err != nil {
		debugctx.Printf(
			ctx,
			"orchestrator ad-hoc request failed method=%q path=%q resolved_path=%q error=%v",
			method,
			endpointPath,
			resolvedPath,
			err,
		)
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator ad-hoc request succeeded method=%q path=%q resolved_path=%q value_type=%T",
		method,
		endpointPath,
		resolvedPath,
		value,
	)
	return value, nil
}

func (r *DefaultOrchestrator) resolveAdHocEndpointPath(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (string, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))

	operation, ok := adHocMetadataOperation(normalizedMethod)
	if !ok {
		return endpointPath, nil
	}

	resourceInfo, err := r.buildResourceInfoForRemoteRead(ctx, endpointPath)
	if err != nil {
		return "", err
	}

	if normalizedMethod == "GET" && shouldResolveAdHocGetAsList(resourceInfo) {
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
			return "", normalizeErr
		}
		resourceInfo.Payload = normalizedBody
	}

	spec, err := r.renderOperationSpec(ctx, resourceInfo, operation, resourceInfo.Payload)
	if err != nil {
		return "", err
	}

	return spec.Path, nil
}

func adHocMetadataOperation(method string) (metadata.Operation, bool) {
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

func shouldResolveAdHocGetAsList(resourceInfo resource.Resource) bool {
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
	// ad-hoc GET, prefer the list path in this case so the raw request targets
	// the correct collection endpoint (for example /components) before fallback.
	return logicalDepth <= templateDepth
}

func (r *DefaultOrchestrator) GetOpenAPISpec(ctx context.Context) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}
	return serverManager.GetOpenAPISpec(ctx)
}
