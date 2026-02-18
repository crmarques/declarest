package orchestrator

import (
	"context"
	"sort"

	"github.com/crmarques/declarest/faults"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

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
	manager, err := r.requireRepository()
	if err != nil {
		return nil, err
	}

	localValue, err := manager.Get(ctx, logicalPath)
	if err != nil {
		if isTypedCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "orchestrator get local miss path=%q", logicalPath)
		} else {
			debugctx.Printf(ctx, "orchestrator get local error path=%q error=%v", logicalPath, err)
		}
		return nil, err
	}

	debugctx.Printf(ctx, "orchestrator get local hit path=%q", logicalPath)
	return localValue, nil
}

func (r *DefaultOrchestrator) GetRemote(ctx context.Context, logicalPath string) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	resourceInfo, infoErr := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if infoErr != nil {
		debugctx.Printf(ctx, "orchestrator get remote preparation failed path=%q error=%v", logicalPath, infoErr)
		return nil, infoErr
	}

	remoteValue, err := serverManager.Get(ctx, resourceInfo)
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
	manager, err := r.requireRepository()
	if err != nil {
		return resource.Resource{}, err
	}

	localValue, err := manager.Get(ctx, logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, localValue)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
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

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
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

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
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

	metadataService, err := r.requireMetadata()
	if err != nil {
		return nil, err
	}
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	resolvedMetadata, err := metadataService.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return nil, err
	}

	items, err := serverManager.List(ctx, normalizedPath, resolvedMetadata)
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
	manager, err := r.requireRepository()
	if err != nil {
		return nil, err
	}

	localValue, err := manager.Get(ctx, logicalPath)
	if err != nil {
		return nil, err
	}

	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, localValue)
	if err != nil {
		return nil, err
	}

	localForCompare, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}
	resourceInfo.Payload = localForCompare

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo)
	if err != nil {
		return nil, err
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
		if items[i].Path == items[j].Path {
			return items[i].Operation < items[j].Operation
		}
		return items[i].Path < items[j].Path
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

	return resource.Normalize(resourceInfo.Payload)
}
