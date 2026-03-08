package orchestrator

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

func (r *Orchestrator) executeRemoteMutation(
	ctx context.Context,
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
) (resource.Resource, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Resource{}, err
	}

	var remotePayload resource.Content
	switch operation {
	case metadata.OperationCreate:
		remotePayload, err = serverManager.Create(ctx, resourceInfo, md)
	case metadata.OperationUpdate:
		remotePayload, err = serverManager.Update(ctx, resourceInfo, md)
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
	descriptor := resourceInfo.PayloadDescriptor
	if remotePayload.Value != nil {
		payload = remotePayload.Value
		descriptor = remotePayload.Descriptor
	}
	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return resource.Resource{}, err
	}

	resourceInfo.Payload = normalizedPayload
	resourceInfo.PayloadDescriptor = descriptor
	return resourceInfo, nil
}

func (r *Orchestrator) resolvePayloadForRemote(
	ctx context.Context,
	logicalPath string,
	content resource.Content,
) (resource.Content, error) {
	if content.Value == nil {
		return content, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, err
	}

	var getSecret func(string) (string, error)
	if r != nil && r.secrets != nil {
		getSecret = func(key string) (string, error) {
			return r.secrets.Get(ctx, key)
		}
	}

	resolved, err := secrets.ResolvePayloadDirectivesForResource(
		content.Value,
		normalizedPath,
		content.Descriptor,
		getSecret,
	)
	if err != nil {
		return resource.Content{}, err
	}
	return resource.Content{
		Value:      resolved,
		Descriptor: content.Descriptor,
	}, nil
}
