package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/support/templatescope"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
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

	if r == nil || r.Secrets == nil {
		return resource.Normalize(value)
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	return secrets.ResolvePayloadForResource(value, normalizedPath, func(key string) (string, error) {
		return r.Secrets.Get(ctx, key)
	})
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

	collectionValue, handled, collectionErr := r.fetchRemoteCollectionValue(ctx, serverManager, resourceInfo)
	if handled {
		if collectionErr != nil {
			return nil, collectionErr
		}
		return collectionValue, nil
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

func (r *DefaultOrchestrator) fetchRemoteCollectionValue(
	ctx context.Context,
	serverManager server.ResourceServer,
	resourceInfo resource.Resource,
) (resource.Value, bool, error) {
	if !r.shouldTreatRemotePathAsCollection(ctx, serverManager, resourceInfo) {
		return nil, false, nil
	}

	items, err := serverManager.List(ctx, resourceInfo.LogicalPath, resourceInfo.Metadata)
	if err != nil {
		// Some APIs incorrectly return 404 for empty collections.
		if isTypedCategory(err, faults.NotFoundError) {
			return []any{}, true, nil
		}
		return nil, true, err
	}

	return listPayloadFromResources(items), true, nil
}

func (r *DefaultOrchestrator) shouldTreatRemotePathAsCollection(
	ctx context.Context,
	serverManager server.ResourceServer,
	resourceInfo resource.Resource,
) bool {
	if r.collectionHintFromRepository(ctx, resourceInfo.LogicalPath) {
		return true
	}

	return r.collectionHintFromOpenAPI(ctx, serverManager, resourceInfo)
}

func (r *DefaultOrchestrator) listOperationTargetsLogicalPath(
	ctx context.Context,
	resourceInfo resource.Resource,
) bool {
	normalizedPath, ok := r.renderedOperationPath(ctx, resourceInfo, metadata.OperationList)
	if !ok {
		return false
	}
	return normalizedPath == resourceInfo.LogicalPath
}

func (r *DefaultOrchestrator) collectionHintFromRepository(
	ctx context.Context,
	logicalPath string,
) bool {
	if r == nil || r.Repository == nil {
		return false
	}

	exists, err := r.Repository.Exists(ctx, logicalPath)
	if err != nil || !exists {
		return false
	}

	_, err = r.Repository.Get(ctx, logicalPath)
	if err == nil {
		return false
	}

	return isTypedCategory(err, faults.NotFoundError)
}

func (r *DefaultOrchestrator) collectionHintFromOpenAPI(
	ctx context.Context,
	serverManager server.ResourceServer,
	resourceInfo resource.Resource,
) bool {
	openAPISpec, err := serverManager.GetOpenAPISpec(ctx)
	if err != nil {
		return false
	}
	existsInOpenAPI, err := metadata.HasOpenAPIPath(resourceInfo.LogicalPath, openAPISpec)
	if err != nil || !existsInOpenAPI {
		return false
	}

	if r.openAPIInferenceHintsCollection(ctx, resourceInfo, resourceInfo.LogicalPath, openAPISpec) {
		return true
	}

	if resourceInfo.LogicalPath == "/" {
		return false
	}

	collectionSelector := strings.TrimSuffix(resourceInfo.LogicalPath, "/") + "/"
	return r.openAPIInferenceHintsCollection(ctx, resourceInfo, collectionSelector, openAPISpec)
}

func (r *DefaultOrchestrator) openAPIInferenceHintsCollection(
	ctx context.Context,
	resourceInfo resource.Resource,
	logicalPath string,
	openAPISpec any,
) bool {
	inferred, err := metadata.InferFromOpenAPISpec(ctx, logicalPath, metadata.InferenceRequest{}, openAPISpec)
	if err != nil {
		return false
	}

	hintInfo := resourceInfo
	hintInfo.Metadata = inferred
	hintInfo.Payload = buildCollectionHintPayload(resourceInfo.Payload, resourceInfo.LogicalPath, inferred)

	if !r.listOperationTargetsLogicalPath(ctx, hintInfo) {
		return false
	}

	if createPath, ok := r.renderedOperationPath(ctx, hintInfo, metadata.OperationCreate); ok && createPath == hintInfo.LogicalPath {
		return true
	}

	getPath, ok := r.renderedOperationPath(ctx, hintInfo, metadata.OperationGet)
	if !ok {
		return false
	}
	return isCollectionItemPath(hintInfo.LogicalPath, getPath)
}

func (r *DefaultOrchestrator) renderedOperationPath(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
) (string, bool) {
	spec, err := r.renderOperationSpec(ctx, resourceInfo, operation, resourceInfo.Payload)
	if err != nil {
		return "", false
	}

	normalizedPath, err := resource.NormalizeLogicalPath(spec.Path)
	if err != nil {
		return "", false
	}
	return normalizedPath, true
}

func isCollectionItemPath(collectionPath string, resourcePath string) bool {
	if collectionPath == "/" {
		return resourcePath != "/" && strings.Count(strings.Trim(resourcePath, "/"), "/") == 0
	}

	trimmedPrefix := strings.TrimSuffix(collectionPath, "/") + "/"
	return strings.HasPrefix(resourcePath, trimmedPrefix)
}

func buildCollectionHintPayload(
	basePayload resource.Value,
	logicalPath string,
	inferred metadata.ResourceMetadata,
) resource.Value {
	payload, _ := basePayload.(map[string]any)
	scope := make(map[string]any, len(payload))
	for key, value := range payload {
		scope[key] = value
	}

	for key, value := range templatescope.DerivePathTemplateFields(logicalPath, inferred) {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if _, exists := scope[key]; exists {
			continue
		}
		scope[key] = value
	}

	if len(scope) == 0 {
		return basePayload
	}
	return scope
}

func listPayloadFromResources(items []resource.Resource) resource.Value {
	if len(items) == 0 {
		return []any{}
	}

	sorted := make([]resource.Resource, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].LogicalPath < sorted[j].LogicalPath
	})

	payload := make([]any, 0, len(sorted))
	for _, item := range sorted {
		payload = append(payload, item.Payload)
	}
	return payload
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
			operationSpec.Path = metadata.EffectiveCollectionPath(metadataCopy, resourceInfo.CollectionPath)
		} else {
			operationSpec.Path = resourceInfo.LogicalPath
		}
		metadataCopy.Operations[string(operation)] = operationSpec
	}

	return metadata.ResolveOperationSpec(ctx, metadataCopy, operation, value)
}
