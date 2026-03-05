package orchestrator

import (
	"context"
	"reflect"
	"sort"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

// Deprecated: prefer the orchestrator.Orchestrator interface methods
// (GetLocal/GetRemote) in new call sites. This convenience method remains for
// concrete-type compatibility and local-then-remote fallback behavior.
func (r *DefaultOrchestrator) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	localValue, err := r.GetLocal(ctx, logicalPath)
	if err == nil {
		return localValue, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return nil, err
	}

	if r == nil || r.server == nil {
		debugctx.Printf(ctx, "orchestrator get local miss path=%q remote_fallback=false", logicalPath)
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get local miss path=%q remote_fallback=true", logicalPath)
	return r.GetRemote(ctx, logicalPath)
}

func (r *DefaultOrchestrator) GetLocal(ctx context.Context, logicalPath string) (resource.Value, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		if faults.IsCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "orchestrator get local miss path=%q", logicalPath)
		} else {
			debugctx.Printf(ctx, "orchestrator get local error path=%q error=%v", logicalPath, err)
		}
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get local hit path=%q", logicalPath)
	return localResource.Payload, nil
}

func (r *DefaultOrchestrator) ResolveLocalResource(ctx context.Context, logicalPath string) (resource.Resource, error) {
	return r.resolveLocalResourceForRead(ctx, logicalPath)
}

func (r *DefaultOrchestrator) GetRemote(ctx context.Context, logicalPath string) (resource.Value, error) {
	resourceInfo, resourceMd, infoErr := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if infoErr != nil {
		debugctx.Printf(ctx, "orchestrator get remote preparation failed path=%q error=%v", logicalPath, infoErr)
		return nil, infoErr
	}

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo, resourceMd)
	if err != nil {
		debugctx.Printf(ctx, "orchestrator get remote error path=%q error=%v", resourceInfo.LogicalPath, err)
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get remote hit path=%q", resourceInfo.LogicalPath)
	return remoteValue, nil
}

func (r *DefaultOrchestrator) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	manager, err := r.requireRepository()
	if err != nil {
		return err
	}
	return manager.Save(ctx, logicalPath, value)
}

func (r *DefaultOrchestrator) Apply(ctx context.Context, logicalPath string, policy orchestrator.ApplyPolicy) (resource.Resource, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	return r.applyDesiredState(ctx, localResource.LogicalPath, localResource.Payload, policy)
}

func (r *DefaultOrchestrator) ApplyWithValue(
	ctx context.Context,
	logicalPath string,
	value resource.Value,
	policy orchestrator.ApplyPolicy,
) (resource.Resource, error) {
	return r.applyDesiredState(ctx, logicalPath, value, policy)
}

func (r *DefaultOrchestrator) applyDesiredState(
	ctx context.Context,
	logicalPath string,
	value resource.Value,
	policy orchestrator.ApplyPolicy,
) (resource.Resource, error) {
	resourceInfo, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo, resourceMd)
	if err != nil {
		if !faults.IsCategory(err, faults.NotFoundError) {
			return resource.Resource{}, err
		}

		item, mutationErr := r.executeRemoteMutation(ctx, resourceInfo, resourceMd, metadata.OperationCreate)
		if mutationErr == nil {
			return item, nil
		}
		if faults.IsCategory(mutationErr, faults.ConflictError) {
			return r.executeRemoteMutation(ctx, resourceInfo, resourceMd, metadata.OperationUpdate)
		}
		return resource.Resource{}, mutationErr
	}

	localForCompare, remoteForCompare, err := r.resolveComparedPayloads(ctx, resourceInfo, resourceMd, resourceInfo.Payload, remoteValue)
	if err != nil {
		return resource.Resource{}, err
	}
	if resolvedRemoteID, ok := resolvedRemoteIDFromPayload(resourceMd, remoteValue); ok {
		resourceInfo.RemoteID = resolvedRemoteID
	}

	if reflect.DeepEqual(localForCompare, remoteForCompare) && !policy.Force {
		normalizedRemote, normalizeErr := resource.Normalize(remoteValue)
		if normalizeErr != nil {
			return resource.Resource{}, normalizeErr
		}
		resourceInfo.Payload = normalizedRemote
		return resourceInfo, nil
	}

	return r.executeRemoteMutation(ctx, resourceInfo, resourceMd, metadata.OperationUpdate)
}

func (r *DefaultOrchestrator) Create(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, resourceMd, metadata.OperationCreate)
}

func (r *DefaultOrchestrator) Update(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, resourceMd, metadata.OperationUpdate)
}

func (r *DefaultOrchestrator) Delete(ctx context.Context, logicalPath string, policy orchestrator.DeletePolicy) error {
	_ = policy

	serverManager, err := r.requireServer()
	if err != nil {
		return err
	}

	resourceInfo, resourceMd, err := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if err != nil {
		return err
	}

	deleteErr := serverManager.Delete(ctx, resourceInfo, resourceMd)
	if deleteErr == nil || !faults.IsCategory(deleteErr, faults.NotFoundError) {
		return deleteErr
	}

	remoteValue, fetchErr := r.fetchRemoteValue(ctx, resourceInfo, resourceMd)
	if fetchErr != nil {
		return fetchErr
	}

	normalizedPayload, normalizeErr := resource.Normalize(remoteValue)
	if normalizeErr != nil {
		return normalizeErr
	}
	resourceInfo.Payload = normalizedPayload

	localAlias, remoteID, identityErr := resolveResourceIdentity(
		resourceInfo.LogicalPath,
		resourceMd,
		normalizedPayload,
	)
	if identityErr != nil {
		return identityErr
	}
	resourceInfo.LocalAlias = localAlias
	resourceInfo.RemoteID = remoteID

	return serverManager.Delete(ctx, resourceInfo, resourceMd)
}

func (r *DefaultOrchestrator) ListLocal(ctx context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	manager, err := r.requireRepository()
	if err != nil {
		return nil, err
	}

	items, err := manager.List(ctx, logicalPath, repository.ListPolicy{Recursive: policy.Recursive})
	if err != nil {
		return nil, err
	}

	for idx := range items {
		if items[idx].Payload != nil {
			continue
		}

		value, getErr := manager.Get(ctx, items[idx].LogicalPath)
		if getErr != nil {
			return nil, getErr
		}
		items[idx].Payload = value
	}

	return items, nil
}

func (r *DefaultOrchestrator) ListRemote(ctx context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return nil, err
	}

	items, err := r.listRemoteResources(ctx, serverManager, normalizedPath, resolvedMetadata)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})

	if policy.Recursive {
		return items, nil
	}

	direct := make([]resource.Resource, 0, len(items))
	for _, item := range items {
		if _, isChild := resource.ChildSegment(normalizedPath, item.LogicalPath); isChild {
			direct = append(direct, item)
		}
	}
	return direct, nil
}

func (r *DefaultOrchestrator) Explain(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	return r.Diff(ctx, logicalPath)
}

func (r *DefaultOrchestrator) Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		return nil, err
	}

	resourceInfo, resourceMd, err := r.buildResourceInfo(ctx, localResource.LogicalPath, localResource.Payload)
	if err != nil {
		return nil, err
	}

	localForCompare, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}
	resourceInfo.Payload = localForCompare

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo, resourceMd)
	if err != nil {
		if faults.IsCategory(err, faults.NotFoundError) {
			remoteValue = nil
		} else {
			return nil, err
		}
	}

	localTransformed, remoteTransformed, err := r.resolveComparedPayloads(ctx, resourceInfo, resourceMd, localForCompare, remoteValue)
	if err != nil {
		return nil, err
	}

	items := buildDiffEntries(resourceInfo.LogicalPath, localTransformed, remoteTransformed)
	sort.Slice(items, func(i int, j int) bool {
		if items[i].ResourcePath == items[j].ResourcePath {
			if items[i].Path == items[j].Path {
				return items[i].Operation < items[j].Operation
			}
			return items[i].Path < items[j].Path
		}
		return items[i].ResourcePath < items[j].ResourcePath
	})
	return items, nil
}

func (r *DefaultOrchestrator) resolveComparedPayloads(
	ctx context.Context,
	resourceInfo resource.Resource,
	resourceMd metadata.ResourceMetadata,
	localValue resource.Value,
	remoteValue resource.Value,
) (resource.Value, resource.Value, error) {
	compareSpec, err := r.renderOperationSpec(ctx, resourceInfo, resourceMd, metadata.OperationCompare, localValue)
	if err != nil {
		return nil, nil, err
	}

	localTransformed, err := applyCompareTransforms(localValue, compareSpec)
	if err != nil {
		return nil, nil, err
	}
	remoteTransformed, err := applyCompareTransforms(remoteValue, compareSpec)
	if err != nil {
		return nil, nil, err
	}

	return localTransformed, remoteTransformed, nil
}

func (r *DefaultOrchestrator) Template(ctx context.Context, logicalPath string, value resource.Value) (resource.Value, error) {
	resourceInfo, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return nil, err
	}

	spec, err := r.renderOperationSpec(ctx, resourceInfo, resourceMd, metadata.OperationUpdate, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}

	if spec.Body != nil {
		return resource.Normalize(spec.Body)
	}

	resolvedPayload, err := secrets.ResolvePayloadDirectivesForResource(
		resourceInfo.Payload,
		resourceInfo.LogicalPath,
		r.effectiveResourceFormat(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	return resource.Normalize(resolvedPayload)
}
