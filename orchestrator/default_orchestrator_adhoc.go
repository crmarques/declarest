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
	operation, ok := adHocMetadataOperation(method)
	if !ok {
		return endpointPath, nil
	}

	resourceInfo, err := r.buildResourceInfoForRemoteRead(ctx, endpointPath)
	if err != nil {
		return "", err
	}

	// Ad-hoc POST targets a collection endpoint. Preserve the requested logical
	// path as the default collection path when metadata does not override it.
	if operation == metadata.OperationCreate {
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
	switch strings.ToUpper(strings.TrimSpace(method)) {
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

func (r *DefaultOrchestrator) GetOpenAPISpec(ctx context.Context) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}
	return serverManager.GetOpenAPISpec(ctx)
}
