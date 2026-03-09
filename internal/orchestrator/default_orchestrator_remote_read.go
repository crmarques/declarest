package orchestrator

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (r *Orchestrator) fetchRemoteValue(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) (resource.Content, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Content{}, err
	}

	remoteValue, err := serverManager.Get(ctx, resolvedResource, md)
	if err == nil {
		ambiguityErr := r.detectRemoteIdentityAmbiguityAfterDirectGet(ctx, serverManager, resolvedResource, md)
		if ambiguityErr != nil {
			return resource.Content{}, ambiguityErr
		}
		return remoteValue, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return resource.Content{}, err
	}

	metadataFallbackValue, metadataHandled, metadataErr := r.fetchRemoteMetadataPathFallbackValue(ctx, serverManager, resolvedResource)
	if metadataHandled {
		if metadataErr != nil {
			return resource.Content{}, metadataErr
		}
		return metadataFallbackValue, nil
	}

	collectionValue, handled, collectionErr := r.fetchRemoteCollectionValue(ctx, serverManager, resolvedResource, md)
	if handled {
		if collectionErr != nil {
			return resource.Content{}, collectionErr
		}
		return collectionValue, nil
	}

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resolvedResource.CollectionPath, md)
	if listErr != nil {
		if faults.IsCategory(listErr, faults.NotFoundError) || isFallbackListPayloadShapeError(listErr) {
			return resource.Content{}, err
		}
		return resource.Content{}, listErr
	}

	matched := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if matchesRemoteFallbackCandidate(resolvedResource, candidate) {
			matched = append(matched, candidate)
		}
	}

	switch len(matched) {
	case 0:
		if allowsSingletonListIdentityFallback(resolvedResource.LogicalPath, md, candidates) {
			return contentFromResource(candidates[0]), nil
		}
		return resource.Content{}, err
	case 1:
		return contentFromResource(matched[0]), nil
	default:
		return resource.Content{}, faults.NewTypedError(
			faults.ConflictError,
			fmt.Sprintf("remote fallback for %q is ambiguous", resolvedResource.LogicalPath),
			nil,
		)
	}
}

func (r *Orchestrator) detectRemoteIdentityAmbiguityAfterDirectGet(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
) error {
	if !shouldCheckRemoteIdentityAmbiguity(resolvedResource, md) {
		return nil
	}

	candidates, listErr := r.listRemoteResources(ctx, serverManager, resolvedResource.CollectionPath, md)
	if listErr != nil {
		if faults.IsCategory(listErr, faults.ConflictError) {
			return listErr
		}
		// Keep direct GET deterministic; this guard is best-effort.
		return nil
	}

	matchCount := 0
	for _, candidate := range candidates {
		if !matchesRemoteFallbackCandidate(resolvedResource, candidate) {
			continue
		}
		matchCount++
		if matchCount > 1 {
			return faults.NewTypedError(
				faults.ConflictError,
				fmt.Sprintf("remote fallback for %q is ambiguous", resolvedResource.LogicalPath),
				nil,
			)
		}
	}

	return nil
}
