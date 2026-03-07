package orchestrator

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) fetchRemoteValue(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	remoteValue, err := serverManager.Get(ctx, resourceInfo, md)
	if err == nil {
		ambiguityErr := r.detectRemoteIdentityAmbiguityAfterDirectGet(ctx, serverManager, resourceInfo, md)
		if ambiguityErr != nil {
			return nil, ambiguityErr
		}
		return remoteValue, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return nil, err
	}

	metadataFallbackValue, metadataHandled, metadataErr := r.fetchRemoteMetadataPathFallbackValue(ctx, serverManager, resourceInfo)
	if metadataHandled {
		if metadataErr != nil {
			return nil, metadataErr
		}
		return metadataFallbackValue, nil
	}

	collectionValue, handled, collectionErr := r.fetchRemoteCollectionValue(ctx, serverManager, resourceInfo, md)
	if handled {
		if collectionErr != nil {
			return nil, collectionErr
		}
		return collectionValue, nil
	}

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resourceInfo.CollectionPath, md)
	if listErr != nil {
		if faults.IsCategory(listErr, faults.NotFoundError) || isFallbackListPayloadShapeError(listErr) {
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
		if allowsSingletonListIdentityFallback(resourceInfo.LogicalPath, md, candidates) {
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
	serverManager managedserver.ManagedServerClient,
	resourceInfo resource.Resource,
	md metadata.ResourceMetadata,
) error {
	if !shouldCheckRemoteIdentityAmbiguity(resourceInfo, md) {
		return nil
	}

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resourceInfo.CollectionPath, md)
	if listErr != nil {
		if faults.IsCategory(listErr, faults.ConflictError) {
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
