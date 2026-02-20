package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/support/identity"
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
		ambiguityErr := r.detectRemoteIdentityAmbiguityAfterDirectGet(ctx, serverManager, resourceInfo)
		if ambiguityErr != nil {
			return nil, ambiguityErr
		}
		return remoteValue, nil
	}
	if !isTypedCategory(err, faults.NotFoundError) {
		return nil, err
	}

	metadataFallbackValue, metadataHandled, metadataErr := r.fetchRemoteMetadataPathFallbackValue(ctx, serverManager, resourceInfo)
	if metadataHandled {
		if metadataErr != nil {
			return nil, metadataErr
		}
		return metadataFallbackValue, nil
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
		if isTypedCategory(listErr, faults.NotFoundError) || isFallbackListPayloadShapeError(listErr) {
			return nil, err
		}
		return nil, listErr
	}

	matched := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if matchesRemoteFallbackCandidate(resourceInfo, candidate) {
			matched = append(matched, candidate)
		}
	}

	switch len(matched) {
	case 0:
		if allowsSingletonListIdentityFallback(resourceInfo.Metadata, candidates) {
			return candidates[0].Payload, nil
		}
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

func (r *DefaultOrchestrator) detectRemoteIdentityAmbiguityAfterDirectGet(
	ctx context.Context,
	serverManager server.ResourceServer,
	resourceInfo resource.Resource,
) error {
	if !shouldCheckRemoteIdentityAmbiguity(resourceInfo) {
		return nil
	}

	candidates, listErr := serverManager.List(ctx, resourceInfo.CollectionPath, resourceInfo.Metadata)
	if listErr != nil {
		if isTypedCategory(listErr, faults.ConflictError) {
			return listErr
		}
		// Keep direct GET deterministic; this guard is best-effort.
		return nil
	}

	matchCount := 0
	for _, candidate := range candidates {
		if !matchesRemoteFallbackCandidate(resourceInfo, candidate) {
			continue
		}
		matchCount++
		if matchCount > 1 {
			return faults.NewTypedError(
				faults.ConflictError,
				fmt.Sprintf("remote fallback for %q is ambiguous", resourceInfo.LogicalPath),
				nil,
			)
		}
	}

	return nil
}

func (r *DefaultOrchestrator) fetchRemoteMetadataPathFallbackValue(
	ctx context.Context,
	serverManager server.ResourceServer,
	resourceInfo resource.Resource,
) (resource.Value, bool, error) {
	visited := map[string]struct{}{
		resourceInfo.LogicalPath: {},
	}
	queue := []string{resourceInfo.LogicalPath}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		if currentPath != resourceInfo.LogicalPath {
			currentInfo, infoErr := r.buildResourceInfoForRemoteRead(ctx, currentPath)
			if infoErr != nil {
				if isTypedCategory(infoErr, faults.ConflictError) {
					return nil, true, infoErr
				}
				continue
			}

			currentValue, currentErr := serverManager.Get(ctx, currentInfo)
			if currentErr == nil {
				return currentValue, true, nil
			}
			// Candidate lookups are best-effort and must not override the original NotFound.
			if isTypedCategory(currentErr, faults.ConflictError) {
				return nil, true, currentErr
			}
		}

		nextPaths, nextErr := r.resolveNextRemoteMetadataFallbackPaths(ctx, serverManager, currentPath)
		if nextErr != nil {
			return nil, true, nextErr
		}

		for _, nextPath := range nextPaths {
			if _, exists := visited[nextPath]; exists {
				continue
			}
			visited[nextPath] = struct{}{}
			queue = append(queue, nextPath)
		}
	}

	return nil, false, nil
}

func (r *DefaultOrchestrator) resolveNextRemoteMetadataFallbackPaths(
	ctx context.Context,
	serverManager server.ResourceServer,
	logicalPath string,
) ([]string, error) {
	segments := splitLogicalPathSegments(logicalPath)
	if len(segments) == 0 {
		return nil, nil
	}

	for segmentIndex := len(segments) - 1; segmentIndex >= 0; segmentIndex-- {
		segmentPath := "/" + strings.Join(segments[:segmentIndex+1], "/")
		segmentInfo, infoErr := r.buildResourceInfoForRemoteRead(ctx, segmentPath)
		if infoErr != nil {
			return nil, infoErr
		}
		if !hasRemoteFallbackIdentityMetadata(segmentInfo.Metadata) {
			continue
		}

		candidates, listErr := serverManager.List(ctx, segmentInfo.CollectionPath, segmentInfo.Metadata)
		if listErr != nil {
			// Fallback list probes are best-effort and must not override the original NotFound.
			if isTypedCategory(listErr, faults.ConflictError) {
				return nil, listErr
			}
			if isTypedCategory(listErr, faults.NotFoundError) ||
				isTypedCategory(listErr, faults.ValidationError) ||
				isFallbackListPayloadShapeError(listErr) {
				continue
			}
			continue
		}

		matched := make([]resource.Resource, 0, len(candidates))
		for _, candidate := range candidates {
			if matchesLocalFallbackIdentity(segmentInfo, candidate.LocalAlias, candidate.RemoteID, candidate.Payload) {
				matched = append(matched, candidate)
			}
		}

		switch len(matched) {
		case 0:
			if allowsSingletonListIdentityFallback(segmentInfo.Metadata, candidates) {
				nextPath, replaced, replaceErr := replaceLogicalPathSegment(
					segments,
					segmentIndex,
					remoteFallbackSegmentValue(candidates[0], segmentInfo.Metadata),
				)
				if replaceErr != nil {
					return nil, replaceErr
				}
				if replaced {
					return []string{nextPath}, nil
				}
			}
			continue
		case 1:
			nextPath, replaced, replaceErr := replaceLogicalPathSegment(
				segments,
				segmentIndex,
				remoteFallbackSegmentValue(matched[0], segmentInfo.Metadata),
			)
			if replaceErr != nil {
				return nil, replaceErr
			}
			if !replaced {
				continue
			}
			return []string{nextPath}, nil
		default:
			return nil, faults.NewTypedError(
				faults.ConflictError,
				fmt.Sprintf("remote fallback for %q is ambiguous", logicalPath),
				nil,
			)
		}
	}

	return nil, nil
}

func remoteFallbackSegmentValue(candidate resource.Resource, md metadata.ResourceMetadata) string {
	if value := strings.TrimSpace(candidate.RemoteID); value != "" {
		return value
	}

	payload, ok := candidate.Payload.(map[string]any)
	if ok {
		for _, attribute := range identityAttributeCandidates(md) {
			value, found := identity.LookupScalarAttribute(payload, attribute)
			if !found || strings.TrimSpace(value) == "" {
				continue
			}
			return strings.TrimSpace(value)
		}
	}
	if value := strings.TrimSpace(candidate.LocalAlias); value != "" {
		return value
	}

	return ""
}

func replaceLogicalPathSegment(
	segments []string,
	segmentIndex int,
	replacement string,
) (string, bool, error) {
	trimmedReplacement := strings.TrimSpace(replacement)
	if trimmedReplacement == "" || trimmedReplacement == segments[segmentIndex] {
		return "", false, nil
	}

	nextSegments := make([]string, len(segments))
	copy(nextSegments, segments)
	nextSegments[segmentIndex] = trimmedReplacement

	nextPath, normalizeErr := resource.NormalizeLogicalPath("/" + strings.Join(nextSegments, "/"))
	if normalizeErr != nil {
		return "", false, normalizeErr
	}
	return nextPath, true, nil
}

func hasRemoteFallbackIdentityMetadata(md metadata.ResourceMetadata) bool {
	return strings.TrimSpace(md.IDFromAttribute) != "" || strings.TrimSpace(md.AliasFromAttribute) != ""
}

func shouldCheckRemoteIdentityAmbiguity(resourceInfo resource.Resource) bool {
	if strings.TrimSpace(resourceInfo.Metadata.IDFromAttribute) == "" {
		return false
	}
	if strings.TrimSpace(resourceInfo.Metadata.AliasFromAttribute) == "" {
		return false
	}

	alias := strings.TrimSpace(resourceInfo.LocalAlias)
	remoteID := strings.TrimSpace(resourceInfo.RemoteID)
	if alias == "" || remoteID == "" {
		return false
	}
	return alias != remoteID
}

func matchesRemoteFallbackCandidate(resourceInfo resource.Resource, candidate resource.Resource) bool {
	if candidate.LocalAlias == resourceInfo.LocalAlias {
		return true
	}
	return resourceInfo.RemoteID != "" && candidate.RemoteID == resourceInfo.RemoteID
}

func allowsSingletonListIdentityFallback(
	md metadata.ResourceMetadata,
	candidates []resource.Resource,
) bool {
	if len(candidates) != 1 {
		return false
	}

	if strings.TrimSpace(md.JQ) != "" {
		return true
	}
	if md.Operations == nil {
		return false
	}

	listSpec, hasListSpec := md.Operations[string(metadata.OperationList)]
	if !hasListSpec {
		return false
	}
	return strings.TrimSpace(listSpec.JQ) != ""
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

func isFallbackListPayloadShapeError(err error) bool {
	if err == nil {
		return false
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	if typedErr.Category != faults.ValidationError {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(typedErr.Message))
	return strings.HasPrefix(message, "list response ") || strings.HasPrefix(message, "list payload ")
}

func (r *DefaultOrchestrator) renderOperationSpec(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
	value resource.Value,
) (metadata.OperationSpec, error) {
	metadataCopy := metadata.CloneResourceMetadata(resourceInfo.Metadata)
	templateResource := resourceInfo
	templateResource.Metadata = metadataCopy
	templateResource.Payload = value

	scope, err := templatescope.BuildResourceScope(templateResource)
	if err != nil {
		return metadata.OperationSpec{}, err
	}

	return metadata.ResolveOperationSpecWithScope(ctx, metadataCopy, operation, scope)
}
