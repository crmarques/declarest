package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	"github.com/crmarques/declarest/server"
)

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

		candidates, listErr := r.listRemoteResources(ctx, serverManager, segmentInfo.CollectionPath, segmentInfo.Metadata)
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
			if allowsSingletonListIdentityFallback(segmentPath, segmentInfo.Metadata, candidates) {
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
	logicalPath string,
	md metadata.ResourceMetadata,
	candidates []resource.Resource,
) bool {
	if len(candidates) != 1 {
		return false
	}
	if !singletonFallbackWithinSelectorDepth(logicalPath, md) {
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

func singletonFallbackWithinSelectorDepth(logicalPath string, md metadata.ResourceMetadata) bool {
	trimmedTemplate := strings.TrimSpace(md.CollectionPath)
	if trimmedTemplate == "" {
		return true
	}

	templateDepth := len(splitLogicalPathSegments(trimmedTemplate))
	if templateDepth == 0 {
		return true
	}

	logicalDepth := len(splitLogicalPathSegments(logicalPath))
	if logicalDepth == 0 {
		return true
	}

	// Limit singleton fallback to selector depth (for example /.../user-registry),
	// and avoid collapsing explicit child identities like /.../user-registry/<name>.
	return logicalDepth <= templateDepth
}
