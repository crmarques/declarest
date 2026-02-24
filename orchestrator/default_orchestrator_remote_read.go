package orchestrator

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

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

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resourceInfo.CollectionPath, resourceInfo.Metadata)
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
		if allowsSingletonListIdentityFallback(resourceInfo.LogicalPath, resourceInfo.Metadata, candidates) {
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

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resourceInfo.CollectionPath, resourceInfo.Metadata)
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
