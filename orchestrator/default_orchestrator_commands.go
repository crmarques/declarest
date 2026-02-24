package orchestrator

import (
	"context"
	"sort"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
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
	if !isTypedCategory(err, faults.NotFoundError) {
		return nil, err
	}

	if r == nil || r.Server == nil {
		debugctx.Printf(ctx, "orchestrator get local miss path=%q remote_fallback=false", logicalPath)
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get local miss path=%q remote_fallback=true", logicalPath)
	return r.GetRemote(ctx, logicalPath)
}

func (r *DefaultOrchestrator) GetLocal(ctx context.Context, logicalPath string) (resource.Value, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		if isTypedCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "orchestrator get local miss path=%q", logicalPath)
		} else {
			debugctx.Printf(ctx, "orchestrator get local error path=%q error=%v", logicalPath, err)
		}
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get local hit path=%q", logicalPath)
	return localResource.Payload, nil
}

func (r *DefaultOrchestrator) GetRemote(ctx context.Context, logicalPath string) (resource.Value, error) {
	resourceInfo, infoErr := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if infoErr != nil {
		debugctx.Printf(ctx, "orchestrator get remote preparation failed path=%q error=%v", logicalPath, infoErr)
		return nil, infoErr
	}

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo)
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

func (r *DefaultOrchestrator) Apply(ctx context.Context, logicalPath string) (resource.Resource, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	resourceInfo, err := r.buildResourceInfo(ctx, localResource.LogicalPath, localResource.Payload)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Resource{}, err
	}

	exists, err := serverManager.Exists(ctx, resourceInfo)
	if err != nil {
		return resource.Resource{}, err
	}

	operation := metadata.OperationCreate
	if exists {
		operation = metadata.OperationUpdate
	}

	return r.executeRemoteMutation(ctx, resourceInfo, operation)
}

func (r *DefaultOrchestrator) Create(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, metadata.OperationCreate)
}

func (r *DefaultOrchestrator) Update(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, metadata.OperationUpdate)
}

func (r *DefaultOrchestrator) Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error {
	_ = policy

	serverManager, err := r.requireServer()
	if err != nil {
		return err
	}

	resourceInfo, err := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if err != nil {
		return err
	}

	deleteErr := serverManager.Delete(ctx, resourceInfo)
	if deleteErr == nil || !isTypedCategory(deleteErr, faults.NotFoundError) {
		return deleteErr
	}

	remoteValue, fetchErr := r.fetchRemoteValue(ctx, resourceInfo)
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
		resourceInfo.Metadata,
		normalizedPayload,
	)
	if identityErr != nil {
		return identityErr
	}
	resourceInfo.LocalAlias = localAlias
	resourceInfo.RemoteID = remoteID

	return serverManager.Delete(ctx, resourceInfo)
}

func (r *DefaultOrchestrator) ListLocal(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error) {
	manager, err := r.requireRepository()
	if err != nil {
		return nil, err
	}

	items, err := manager.List(ctx, logicalPath, repository.ListPolicy{Recursive: policy.Recursive})
	if err != nil {
		return nil, err
	}

	// Keep local list output parity with remote list by including each resource payload.
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

func (r *DefaultOrchestrator) ListRemote(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error) {
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
		if isDirectChildPath(normalizedPath, item.LogicalPath) {
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

	resourceInfo, err := r.buildResourceInfo(ctx, localResource.LogicalPath, localResource.Payload)
	if err != nil {
		return nil, err
	}

	localForCompare, err := r.resolvePayloadForRemote(ctx, resourceInfo.LogicalPath, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}
	resourceInfo.Payload = localForCompare

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo)
	if err != nil {
		// A missing remote resource represents full drift from desired local state.
		// Keep diff deterministic by comparing against a nil remote payload.
		if isTypedCategory(err, faults.NotFoundError) {
			remoteValue = nil
		} else {
			return nil, err
		}
	}

	compareSpec, err := r.renderOperationSpec(ctx, resourceInfo, metadata.OperationCompare, localForCompare)
	if err != nil {
		return nil, err
	}

	localTransformed, err := applyCompareTransforms(localForCompare, compareSpec)
	if err != nil {
		return nil, err
	}
	remoteTransformed, err := applyCompareTransforms(remoteValue, compareSpec)
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

func (r *DefaultOrchestrator) Template(ctx context.Context, logicalPath string, value resource.Value) (resource.Value, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return nil, err
	}

	spec, err := r.renderOperationSpec(ctx, resourceInfo, metadata.OperationUpdate, resourceInfo.Payload)
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
