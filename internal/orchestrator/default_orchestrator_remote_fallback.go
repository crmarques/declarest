package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *Orchestrator) fetchRemoteMetadataPathFallbackValue(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
) (resource.Content, bool, error) {
	visited := map[string]struct{}{
		resolvedResource.LogicalPath: {},
	}
	queue := []string{resolvedResource.LogicalPath}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		if currentPath != resolvedResource.LogicalPath {
			currentInfo, currentMd, infoErr := r.buildResourceInfoForRemoteRead(ctx, currentPath)
			if infoErr != nil {
				if faults.IsCategory(infoErr, faults.ConflictError) {
					return resource.Content{}, true, infoErr
				}
				continue
			}

			currentValue, currentErr := serverManager.Get(ctx, currentInfo, currentMd)
			if currentErr == nil {
				return currentValue, true, nil
			}
			// Candidate lookups are best-effort and must not override the original NotFound.
			if faults.IsCategory(currentErr, faults.ConflictError) {
				return resource.Content{}, true, currentErr
			}
		}

		nextPaths, nextErr := r.resolveNextRemoteMetadataFallbackPaths(ctx, serverManager, currentPath)
		if nextErr != nil {
			return resource.Content{}, true, nextErr
		}

		for _, nextPath := range nextPaths {
			if _, exists := visited[nextPath]; exists {
				continue
			}
			visited[nextPath] = struct{}{}
			queue = append(queue, nextPath)
		}
	}

	return resource.Content{}, false, nil
}

func (r *Orchestrator) resolveNextRemoteMetadataFallbackPaths(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	logicalPath string,
) ([]string, error) {
	segments := resource.SplitRawPathSegments(logicalPath)
	if len(segments) == 0 {
		return nil, nil
	}

	for segmentIndex := len(segments) - 1; segmentIndex >= 0; segmentIndex-- {
		segmentPath := "/" + strings.Join(segments[:segmentIndex+1], "/")
		segmentInfo, segmentMd, infoErr := r.buildResourceInfoForRemoteRead(ctx, segmentPath)
		if infoErr != nil {
			return nil, infoErr
		}
		if !hasRemoteFallbackIdentityMetadata(segmentMd) {
			continue
		}

		candidates, listErr := r.listRemoteResources(ctx, serverManager, segmentInfo.CollectionPath, segmentMd)
		if listErr != nil {
			// Fallback list probes are best-effort and must not override the original NotFound.
			if faults.IsCategory(listErr, faults.ConflictError) {
				return nil, listErr
			}
			if faults.IsCategory(listErr, faults.NotFoundError) ||
				faults.IsCategory(listErr, faults.ValidationError) ||
				isFallbackListPayloadShapeError(listErr) {
				continue
			}
			continue
		}

		matched := make([]resource.Resource, 0, len(candidates))
		for _, candidate := range candidates {
			if matchesFallbackCandidate(segmentInfo, candidate) {
				matched = append(matched, candidate)
			}
		}

		switch len(matched) {
		case 0:
			if allowsSingletonListIdentityFallback(segmentPath, segmentMd, candidates) {
				nextPath, replaced, replaceErr := replaceLogicalPathSegment(
					segments,
					segmentIndex,
					fallbackSegmentValue(candidates[0]),
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
				fallbackSegmentValue(matched[0]),
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
	return strings.TrimSpace(md.ID) != "" || strings.TrimSpace(md.Alias) != ""
}

func shouldCheckRemoteIdentityAmbiguity(resolvedResource resource.Resource, md metadata.ResourceMetadata) bool {
	if strings.TrimSpace(md.ID) == "" {
		return false
	}
	if strings.TrimSpace(md.Alias) == "" {
		return false
	}

	alias := strings.TrimSpace(resolvedResource.LocalAlias)
	remoteID := strings.TrimSpace(resolvedResource.RemoteID)
	if alias == "" || remoteID == "" {
		return false
	}
	return alias != remoteID
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

	if metadata.HasTransformJQ(md.Transforms) {
		return true
	}
	if md.Operations == nil {
		return false
	}

	listSpec, hasListSpec := md.Operations[string(metadata.OperationList)]
	if !hasListSpec {
		return false
	}
	return metadata.HasTransformJQ(listSpec.Transforms)
}

func singletonFallbackWithinSelectorDepth(logicalPath string, md metadata.ResourceMetadata) bool {
	trimmedTemplate := strings.TrimSpace(md.RemoteCollectionPath)
	if trimmedTemplate == "" {
		return true
	}

	templateDepth := len(resource.SplitRawPathSegments(trimmedTemplate))
	if templateDepth == 0 {
		return true
	}

	logicalDepth := len(resource.SplitRawPathSegments(logicalPath))
	if logicalDepth == 0 {
		return true
	}

	// Limit singleton fallback to selector depth (for example /.../user-registry),
	// and avoid collapsing explicit child identities like /.../user-registry/<name>.
	return logicalDepth <= templateDepth
}
