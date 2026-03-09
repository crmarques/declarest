package orchestrator

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

func (r *Orchestrator) GetLocal(ctx context.Context, logicalPath string) (resource.Content, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		if faults.IsCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "orchestrator get local miss path=%q", logicalPath)
		} else {
			debugctx.Printf(ctx, "orchestrator get local error path=%q error=%v", logicalPath, err)
		}
		return resource.Content{}, err
	}

	debugctx.Printf(ctx, "orchestrator get local hit path=%q", logicalPath)
	return contentFromResource(localResource), nil
}

func (r *Orchestrator) ResolveLocalResource(ctx context.Context, logicalPath string) (resource.Resource, error) {
	return r.resolveLocalResourceForRead(ctx, logicalPath)
}

func (r *Orchestrator) GetRemote(ctx context.Context, logicalPath string) (resource.Content, error) {
	resolvedResource, resourceMd, infoErr := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if infoErr != nil {
		debugctx.Printf(ctx, "orchestrator get remote preparation failed path=%q error=%v", logicalPath, infoErr)
		return resource.Content{}, infoErr
	}

	remoteValue, err := r.fetchRemoteValue(ctx, resolvedResource, resourceMd)
	if err != nil {
		debugctx.Printf(ctx, "orchestrator get remote error path=%q error=%v", resolvedResource.LogicalPath, err)
		return resource.Content{}, err
	}

	debugctx.Printf(ctx, "orchestrator get remote hit path=%q", resolvedResource.LogicalPath)
	return remoteValue, nil
}

func (r *Orchestrator) Save(ctx context.Context, logicalPath string, content resource.Content) error {
	manager, err := r.requireRepository()
	if err != nil {
		return err
	}
	return r.saveLocalResource(ctx, manager, logicalPath, content)
}

func (r *Orchestrator) Apply(ctx context.Context, logicalPath string, policy orchestrator.ApplyPolicy) (resource.Resource, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	return r.applyDesiredState(ctx, localResource.LogicalPath, contentFromResource(localResource), policy)
}

func (r *Orchestrator) ApplyWithContent(
	ctx context.Context,
	logicalPath string,
	content resource.Content,
	policy orchestrator.ApplyPolicy,
) (resource.Resource, error) {
	return r.applyDesiredState(ctx, logicalPath, content, policy)
}

func (r *Orchestrator) applyDesiredState(
	ctx context.Context,
	logicalPath string,
	content resource.Content,
	policy orchestrator.ApplyPolicy,
) (resource.Resource, error) {
	resolvedResource, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, content)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resolvedResource.LogicalPath, contentFromResource(resolvedResource))
	if err != nil {
		return resource.Resource{}, err
	}
	resolvedResource.Payload = resolvedPayload.Value
	resolvedResource.PayloadDescriptor = resolvedPayload.Descriptor

	remoteValue, err := r.fetchRemoteValue(ctx, resolvedResource, resourceMd)
	if err != nil {
		if !faults.IsCategory(err, faults.NotFoundError) {
			return resource.Resource{}, err
		}

		item, mutationErr := r.executeRemoteMutation(ctx, resolvedResource, resourceMd, metadata.OperationCreate)
		if mutationErr == nil {
			return item, nil
		}
		if faults.IsCategory(mutationErr, faults.ConflictError) {
			return r.executeRemoteMutation(ctx, resolvedResource, resourceMd, metadata.OperationUpdate)
		}
		return resource.Resource{}, mutationErr
	}

	localForCompare, remoteForCompare, err := r.resolveComparedPayloads(
		ctx,
		resolvedResource,
		resourceMd,
		contentFromResource(resolvedResource),
		remoteValue,
	)
	if err != nil {
		return resource.Resource{}, err
	}
	if resolvedRemoteID, ok := resolvedRemoteIDFromPayload(resourceMd, remoteValue.Value); ok {
		resolvedResource.RemoteID = resolvedRemoteID
	}

	if reflect.DeepEqual(localForCompare, remoteForCompare) && !policy.Force {
		normalizedRemote, normalizeErr := resource.Normalize(remoteValue.Value)
		if normalizeErr != nil {
			return resource.Resource{}, normalizeErr
		}
		resolvedResource.Payload = normalizedRemote
		resolvedResource.PayloadDescriptor = remoteValue.Descriptor
		return resolvedResource, nil
	}

	return r.executeRemoteMutation(ctx, resolvedResource, resourceMd, metadata.OperationUpdate)
}

func (r *Orchestrator) Create(ctx context.Context, logicalPath string, content resource.Content) (resource.Resource, error) {
	resolvedResource, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, content)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resolvedResource.LogicalPath, contentFromResource(resolvedResource))
	if err != nil {
		return resource.Resource{}, err
	}
	resolvedResource.Payload = resolvedPayload.Value
	resolvedResource.PayloadDescriptor = resolvedPayload.Descriptor

	return r.executeRemoteMutation(ctx, resolvedResource, resourceMd, metadata.OperationCreate)
}

func (r *Orchestrator) Update(ctx context.Context, logicalPath string, content resource.Content) (resource.Resource, error) {
	resolvedResource, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, content)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resolvedResource.LogicalPath, contentFromResource(resolvedResource))
	if err != nil {
		return resource.Resource{}, err
	}
	resolvedResource.Payload = resolvedPayload.Value
	resolvedResource.PayloadDescriptor = resolvedPayload.Descriptor

	return r.executeRemoteMutation(ctx, resolvedResource, resourceMd, metadata.OperationUpdate)
}

func (r *Orchestrator) Delete(ctx context.Context, logicalPath string, _ orchestrator.DeletePolicy) error {
	serverManager, err := r.requireServer()
	if err != nil {
		return err
	}

	resolvedResource, resourceMd, err := r.buildResourceInfoForRemoteRead(ctx, logicalPath)
	if err != nil {
		return err
	}

	deleteErr := serverManager.Delete(ctx, resolvedResource, resourceMd)
	if deleteErr == nil || !faults.IsCategory(deleteErr, faults.NotFoundError) {
		return deleteErr
	}

	remoteValue, fetchErr := r.fetchRemoteValue(ctx, resolvedResource, resourceMd)
	if fetchErr != nil {
		return fetchErr
	}

	normalizedPayload, normalizeErr := resource.Normalize(remoteValue.Value)
	if normalizeErr != nil {
		return normalizeErr
	}
	resolvedResource.Payload = normalizedPayload
	resolvedResource.PayloadDescriptor = remoteValue.Descriptor

	localAlias, remoteID, identityErr := resolveResourceIdentity(
		resolvedResource.LogicalPath,
		resourceMd,
		normalizedPayload,
	)
	if identityErr != nil {
		return identityErr
	}
	resolvedResource.LocalAlias = localAlias
	resolvedResource.RemoteID = remoteID

	return serverManager.Delete(ctx, resolvedResource, resourceMd)
}

func (r *Orchestrator) ListLocal(ctx context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
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

		content, getErr := manager.Get(ctx, items[idx].LogicalPath)
		if getErr != nil {
			return nil, getErr
		}
		items[idx].Payload = content.Value
		items[idx].PayloadDescriptor = content.Descriptor
	}

	return items, nil
}

func (r *Orchestrator) ListRemote(ctx context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
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

func (r *Orchestrator) Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	localResource, err := r.resolveLocalResourceForRead(ctx, logicalPath)
	if err != nil {
		return nil, err
	}

	resolvedResource, resourceMd, err := r.buildResourceInfo(ctx, localResource.LogicalPath, contentFromResource(localResource))
	if err != nil {
		return nil, err
	}

	localForCompare, err := r.resolvePayloadForRemote(ctx, resolvedResource.LogicalPath, contentFromResource(resolvedResource))
	if err != nil {
		return nil, err
	}
	resolvedResource.Payload = localForCompare.Value
	resolvedResource.PayloadDescriptor = localForCompare.Descriptor

	remoteValue, err := r.fetchRemoteValue(ctx, resolvedResource, resourceMd)
	if err != nil {
		if faults.IsCategory(err, faults.NotFoundError) {
			remoteValue = resource.Content{}
		} else {
			return nil, err
		}
	}

	var remotePayload resource.Value
	if remoteValue.Value != nil {
		remotePayload = remoteValue.Value
	}
	localTransformed, remoteTransformed, err := r.resolveComparedPayloads(
		ctx,
		resolvedResource,
		resourceMd,
		localForCompare,
		resource.Content{Value: remotePayload, Descriptor: remoteValue.Descriptor},
	)
	if err != nil {
		return nil, err
	}

	items := buildDiffEntries(resolvedResource.LogicalPath, localTransformed, remoteTransformed)
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

func (r *Orchestrator) resolveComparedPayloads(
	ctx context.Context,
	resolvedResource resource.Resource,
	resourceMd metadata.ResourceMetadata,
	localContent resource.Content,
	remoteContent resource.Content,
) (resource.Value, resource.Value, error) {
	compareSpec, err := r.renderOperationSpec(ctx, resolvedResource, resourceMd, metadata.OperationCompare, localContent.Value)
	if err != nil {
		return nil, nil, err
	}

	payloadType := strings.TrimSpace(localContent.Descriptor.PayloadType)
	if payloadType == "" {
		payloadType = strings.TrimSpace(resolvedResource.PayloadDescriptor.PayloadType)
	}
	if payloadType == "" {
		payloadType = strings.TrimSpace(resourceMd.PayloadType)
	}
	if payloadType == "" {
		payloadType = resource.PayloadTypeJSON
	}
	payloadType, payloadTypeErr := metadata.ValidateResourceFormat(payloadType)
	if payloadTypeErr != nil {
		return nil, nil, payloadTypeErr
	}
	if !resource.IsStructuredPayloadType(payloadType) {
		if len(compareSpec.Transforms) > 0 {
			if !resourceMd.IsWholeResourceSecret() {
				return nil, nil, faults.NewValidationError(
					fmt.Sprintf("compare transforms require structured payloads, got %q", payloadType),
					nil,
				)
			}

			localInput := compareInputForWholeResourceOpaquePayload(resolvedResource, resourceMd, localContent.Descriptor)
			localTransformed, err := applyCompareTransforms(localInput, compareSpec)
			if err != nil {
				return nil, nil, err
			}

			if remoteContent.Value == nil {
				return localTransformed, nil, nil
			}

			remoteInput := remoteContent.Value
			if !isStructuredCompareValue(remoteInput) {
				remoteInput = compareInputForWholeResourceOpaquePayload(resolvedResource, resourceMd, remoteContent.Descriptor)
			}
			remoteTransformed, err := applyCompareTransforms(remoteInput, compareSpec)
			if err != nil {
				return nil, nil, err
			}

			return localTransformed, remoteTransformed, nil
		}
		normalizedLocal, err := resource.Normalize(localContent.Value)
		if err != nil {
			return nil, nil, err
		}
		normalizedRemote, err := resource.Normalize(remoteContent.Value)
		if err != nil {
			return nil, nil, err
		}
		return normalizedLocal, normalizedRemote, nil
	}

	localTransformed, err := applyCompareTransforms(localContent.Value, compareSpec)
	if err != nil {
		return nil, nil, err
	}
	remoteTransformed, err := applyCompareTransforms(remoteContent.Value, compareSpec)
	if err != nil {
		return nil, nil, err
	}

	return localTransformed, remoteTransformed, nil
}

func compareInputForWholeResourceOpaquePayload(
	resolvedResource resource.Resource,
	resourceMd metadata.ResourceMetadata,
	descriptor resource.PayloadDescriptor,
) resource.Value {
	resolved := descriptor
	if !resource.IsPayloadDescriptorExplicit(resolved) && strings.TrimSpace(resourceMd.PayloadType) != "" {
		resolved = resource.PayloadDescriptor{PayloadType: resourceMd.PayloadType}
	}
	resolved = resource.NormalizePayloadDescriptor(resolved)
	input := map[string]any{}

	if id := strings.TrimSpace(resolvedResource.RemoteID); id != "" {
		input["id"] = id
	}
	if alias := strings.TrimSpace(resolvedResource.LocalAlias); alias != "" {
		input["alias"] = alias
		input["name"] = alias
	}
	if payloadType := strings.TrimSpace(resolved.PayloadType); payloadType != "" {
		input["payloadType"] = payloadType
	}
	if contentType := strings.TrimSpace(resolved.MediaType); contentType != "" {
		input["contentType"] = contentType
	}
	if extension := strings.TrimSpace(resolved.Extension); extension != "" {
		input["payloadExtension"] = extension
	}

	return input
}

func isStructuredCompareValue(value resource.Value) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func (r *Orchestrator) Template(ctx context.Context, logicalPath string, content resource.Content) (resource.Content, error) {
	resolvedResource, resourceMd, err := r.buildResourceInfo(ctx, logicalPath, content)
	if err != nil {
		return resource.Content{}, err
	}

	spec, err := r.renderOperationSpec(ctx, resolvedResource, resourceMd, metadata.OperationUpdate, resolvedResource.Payload)
	if err != nil {
		return resource.Content{}, err
	}

	if spec.Body != nil {
		bodyValue := spec.Body
		if typed, ok := spec.Body.(resource.Content); ok {
			bodyValue = typed.Value
		}
		normalizedBody, normalizeErr := resource.Normalize(bodyValue)
		if normalizeErr != nil {
			return resource.Content{}, normalizeErr
		}
		return resource.Content{Value: normalizedBody, Descriptor: resolvedResource.PayloadDescriptor}, nil
	}

	resolvedPayload, err := secrets.ResolvePayloadDirectivesForResource(
		resolvedResource.Payload,
		resolvedResource.LogicalPath,
		resolvedResource.PayloadDescriptor,
		nil,
	)
	if err != nil {
		return resource.Content{}, err
	}

	normalizedPayload, err := resource.Normalize(resolvedPayload)
	if err != nil {
		return resource.Content{}, err
	}
	return resource.Content{Value: normalizedPayload, Descriptor: resolvedResource.PayloadDescriptor}, nil
}
