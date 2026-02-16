package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) executeRemoteMutation(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
) (resource.Resource, error) {
	manager, err := r.requireRepository()
	if err != nil {
		return resource.Resource{}, err
	}
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Resource{}, err
	}

	var remotePayload resource.Value
	switch operation {
	case metadata.OperationCreate:
		remotePayload, err = serverManager.Create(ctx, resourceInfo)
	case metadata.OperationUpdate:
		remotePayload, err = serverManager.Update(ctx, resourceInfo)
	default:
		return resource.Resource{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported remote mutation operation %q", operation),
			nil,
		)
	}
	if err != nil {
		return resource.Resource{}, err
	}

	payloadForLocal := resourceInfo.Payload
	if remotePayload != nil {
		payloadForLocal = remotePayload
	}

	maskedPayload, err := r.maskPayloadForLocal(ctx, payloadForLocal)
	if err != nil {
		return resource.Resource{}, err
	}

	if err := manager.Save(ctx, resourceInfo.LogicalPath, maskedPayload); err != nil {
		return resource.Resource{}, err
	}

	resourceInfo.Payload = maskedPayload
	return resourceInfo, nil
}

func (r *DefaultOrchestrator) resolvePayloadForRemote(ctx context.Context, value resource.Value) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	if r == nil || r.Secrets == nil {
		return resource.Normalize(value)
	}

	return r.Secrets.ResolvePayload(ctx, value)
}

func (r *DefaultOrchestrator) maskPayloadForLocal(ctx context.Context, value resource.Value) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	if r == nil || r.Secrets == nil {
		return resource.Normalize(value)
	}

	return r.Secrets.MaskPayload(ctx, value)
}

func (r *DefaultOrchestrator) fetchRemoteValue(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	remoteValue, err := serverManager.Get(ctx, resourceInfo)
	if err == nil {
		return remoteValue, nil
	}
	if !isTypedCategory(err, faults.NotFoundError) {
		return nil, err
	}

	candidates, listErr := serverManager.List(ctx, resourceInfo.CollectionPath, resourceInfo.Metadata)
	if listErr != nil {
		return nil, listErr
	}

	matched := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.LocalAlias == resourceInfo.LocalAlias {
			matched = append(matched, candidate)
			continue
		}
		if resourceInfo.RemoteID != "" && candidate.RemoteID == resourceInfo.RemoteID {
			matched = append(matched, candidate)
		}
	}

	switch len(matched) {
	case 0:
		return nil, err
	case 1:
		return matched[0].Payload, nil
	default:
		return nil, faults.NewTypedError(
			faults.ConflictError,
			fmt.Sprintf("remote fallback for %q is ambiguous", resourceInfo.LogicalPath),
			nil,
		)
	}
}

func (r *DefaultOrchestrator) renderOperationSpec(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
	value resource.Value,
) (metadata.OperationSpec, error) {
	metadataCopy := metadata.CloneResourceMetadata(resourceInfo.Metadata)
	if metadataCopy.Operations == nil {
		metadataCopy.Operations = map[string]metadata.OperationSpec{}
	}

	operationSpec := metadataCopy.Operations[string(operation)]
	if strings.TrimSpace(operationSpec.Path) == "" {
		if operation == metadata.OperationList {
			operationSpec.Path = resourceInfo.CollectionPath
		} else {
			operationSpec.Path = resourceInfo.LogicalPath
		}
		metadataCopy.Operations[string(operation)] = operationSpec
	}

	return metadata.ResolveOperationSpec(ctx, metadataCopy, operation, value)
}
