package orchestrator

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) fetchRemoteCollectionValue(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
) (resource.Value, bool, error) {
	if !r.shouldTreatRemotePathAsCollection(ctx, serverManager, resourceInfo) {
		return nil, false, nil
	}

	items, err := r.listRemoteResources(ctx, serverManager, resourceInfo.LogicalPath, md)
	if err != nil {
		// Some APIs incorrectly return 404 for empty collections.
		if faults.IsCategory(err, faults.NotFoundError) {
			if r.isMissingParentForCollectionNotFound(ctx, serverManager, resourceInfo) {
				return nil, true, err
			}
			return []any{}, true, nil
		}
		if isFallbackListPayloadShapeError(err) {
			return nil, false, nil
		}
		return nil, true, err
	}

	return listPayloadFromResources(items), true, nil
}

func (r *DefaultOrchestrator) withListJQResourceResolver(ctx context.Context) context.Context {
	return managedserver.WithListJQResourceResolver(ctx, r.resolveListJQResource)
}

func (r *DefaultOrchestrator) listRemoteResources(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	collectionPath string,
	md metadata.ResourceMetadata,
) ([]resource.Resource, error) {
	return serverManager.List(r.withListJQResourceResolver(ctx), collectionPath, md)
}

func (r *DefaultOrchestrator) resolveListJQResource(
	ctx context.Context,
	logicalPath string,
) (resource.Value, error) {
	return r.GetRemote(ctx, logicalPath)
}

func (r *DefaultOrchestrator) shouldTreatRemotePathAsCollection(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
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
	md metadata.ResourceMetadata,
) bool {
	normalizedPath, ok := r.renderedOperationPath(ctx, resourceInfo, md, metadata.OperationList)
	if !ok {
		return false
	}
	return normalizedPath == resourceInfo.LogicalPath
}

func (r *DefaultOrchestrator) collectionHintFromRepository(
	ctx context.Context,
	logicalPath string,
) bool {
	if r == nil || r.repository == nil {
		return false
	}

	exists, err := r.repository.Exists(ctx, logicalPath)
	if err != nil || !exists {
		return false
	}

	_, err = r.repository.Get(ctx, logicalPath)
	if err == nil {
		return false
	}

	return faults.IsCategory(err, faults.NotFoundError)
}

func (r *DefaultOrchestrator) collectionHintFromOpenAPI(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
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

	// Avoid treating concrete resource paths with child endpoints as collections
	// (for example /admin/realms/{realm}) when probing a synthetic trailing-slash
	// variant for collection hints.
	if openAPIExactPathLooksLikeResource(openAPISpec, resourceInfo.LogicalPath) {
		return false
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
	hintInfo.Payload = buildCollectionHintPayload(resourceInfo.Payload, resourceInfo.LogicalPath, inferred)

	if !r.listOperationTargetsLogicalPath(ctx, hintInfo, inferred) {
		return false
	}

	if createPath, ok := r.renderedOperationPath(ctx, hintInfo, inferred, metadata.OperationCreate); ok && createPath == hintInfo.LogicalPath {
		return true
	}

	getPath, ok := r.renderedOperationPath(ctx, hintInfo, inferred, metadata.OperationGet)
	if !ok {
		return false
	}
	return isCollectionItemPath(hintInfo.LogicalPath, getPath)
}

func (r *DefaultOrchestrator) renderedOperationPath(
	ctx context.Context,
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
) (string, bool) {
	spec, err := r.renderOperationSpec(ctx, resourceInfo, md, operation, resourceInfo.Payload)
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

func openAPIExactPathLooksLikeResource(openAPISpec any, logicalPath string) bool {
	root, ok := openAPISpec.(map[string]any)
	if !ok {
		return false
	}

	pathsValue, found := root["paths"]
	if !found {
		return false
	}
	paths, ok := pathsValue.(map[string]any)
	if !ok {
		return false
	}

	targetSegments := resource.SplitRawPathSegments(logicalPath)
	if len(targetSegments) == 0 {
		return false
	}

	for rawPath, pathItemValue := range paths {
		candidateSegments := resource.SplitRawPathSegments(rawPath)
		if len(candidateSegments) != len(targetSegments) {
			continue
		}
		if !matchesOpenAPIPathSegments(candidateSegments, targetSegments) {
			continue
		}

		pathItem, ok := pathItemValue.(map[string]any)
		if !ok {
			return false
		}

		hasGet := false
		hasPost := false
		hasResourceMutation := false
		for method := range pathItem {
			switch strings.ToLower(strings.TrimSpace(method)) {
			case "get":
				hasGet = true
			case "post":
				hasPost = true
			case "put", "patch", "delete":
				hasResourceMutation = true
			}
		}

		return hasGet && hasResourceMutation && !hasPost
	}

	return false
}

func matchesOpenAPIPathSegments(candidate []string, target []string) bool {
	if len(candidate) != len(target) {
		return false
	}

	for idx := range candidate {
		candidateSegment := strings.TrimSpace(candidate[idx])
		targetSegment := strings.TrimSpace(target[idx])
		if candidateSegment == targetSegment {
			continue
		}
		if strings.HasPrefix(candidateSegment, "{") && strings.HasSuffix(candidateSegment, "}") {
			if targetSegment == "" {
				return false
			}
			continue
		}
		return false
	}

	return true
}

func (r *DefaultOrchestrator) isMissingParentForCollectionNotFound(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resourceInfo resource.Resource,
) bool {
	parentPath := path.Dir(strings.TrimSuffix(resourceInfo.LogicalPath, "/"))
	if parentPath == "." || parentPath == "" || parentPath == "/" {
		return false
	}

	parentInfo, parentMd, err := r.buildResourceInfoForRemoteRead(ctx, parentPath)
	if err != nil {
		return false
	}

	_, err = serverManager.Get(ctx, parentInfo, parentMd)
	return faults.IsCategory(err, faults.NotFoundError)
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

func isFallbackListPayloadShapeError(err error) bool {
	return managedserver.IsListPayloadShapeError(err)
}

func (r *DefaultOrchestrator) renderOperationSpec(
	ctx context.Context,
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
	value resource.Value,
) (metadata.OperationSpec, error) {
	metadataCopy := metadata.CloneResourceMetadata(md)
	templateResource := resourceInfo
	templateResource.Payload = value

	if renderer, ok := r.metadata.(metadata.ResourceOperationSpecRenderer); ok {
		spec, err := renderer.RenderOperationSpecForResource(ctx, metadata.ResourceOperationSpecInput{
			LogicalPath:    templateResource.LogicalPath,
			CollectionPath: templateResource.CollectionPath,
			LocalAlias:     templateResource.LocalAlias,
			RemoteID:       templateResource.RemoteID,
			Metadata:       metadataCopy,
			Payload:        templateResource.Payload,
		}, operation)
		if err != nil {
			return metadata.OperationSpec{}, err
		}
		if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
			spec.Method = overrideMethod
		}
		return spec, nil
	}

	scope, err := templatescope.BuildResourceScope(templateResource, metadataCopy)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	applyOrchestratorPayloadScope(scope, metadataCopy, r.effectiveResourceFormat())

	spec, err := metadata.ResolveOperationSpecWithScope(ctx, metadataCopy, operation, scope)
	if err != nil {
		return metadata.OperationSpec{}, err
	}
	if overrideMethod, ok := metadata.OperationHTTPMethodOverride(ctx, operation); ok {
		spec.Method = overrideMethod
	}
	return spec, nil
}

func applyOrchestratorPayloadScope(scope map[string]any, md metadata.ResourceMetadata, fallback string) {
	if scope == nil {
		return
	}

	scope["resourceFormat"] = metadata.NormalizeResourceFormat(fallback)

	payloadType, err := metadata.EffectivePayloadType(md, fallback)
	if err != nil {
		payloadType = metadata.NormalizeResourceFormat(fallback)
	}
	scope["payloadType"] = payloadType

	if mediaType, mediaErr := metadata.ResourceFormatMediaType(payloadType); mediaErr == nil {
		scope["payloadMediaType"] = mediaType
	}
	if extension, extensionErr := metadata.ResourceFormatExtension(payloadType); extensionErr == nil {
		scope["payloadExtension"] = extension
	}
}
