package orchestrator

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

func (r *DefaultOrchestrator) executeRemoteMutation(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
) (resource.Resource, error) {
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

	payload := resourceInfo.Payload
	if remotePayload != nil {
		payload = remotePayload
	}
	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return resource.Resource{}, err
	}

	resourceInfo.Payload = normalizedPayload
	return resourceInfo, nil
}

func (r *DefaultOrchestrator) resolvePayloadForRemote(
	ctx context.Context,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	var getSecret func(string) (string, error)
	if r != nil && r.Secrets != nil {
		getSecret = func(key string) (string, error) {
			return r.Secrets.Get(ctx, key)
		}
	}

	return secrets.ResolvePayloadDirectivesForResource(
		value,
		normalizedPath,
		r.effectiveResourceFormat(),
		getSecret,
	)
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
