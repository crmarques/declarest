package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/resourceexternalization"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) saveLocalResource(
	ctx context.Context,
	manager repository.ResourceStore,
	logicalPath string,
	value resource.Value,
) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return err
	}

	entries, err := metadata.ResolveExternalizedAttributes(resolvedMetadata)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return manager.Save(ctx, normalizedPath, value)
	}

	artifactStore, ok := manager.(repository.ResourceArtifactStore)
	if !ok {
		return faults.NewTypedError(
			faults.InternalError,
			"repository store does not support resource artifacts",
			nil,
		)
	}

	extracted, err := resourceexternalization.Extract(value, entries)
	if err != nil {
		return err
	}

	return artifactStore.SaveResourceWithArtifacts(ctx, normalizedPath, extracted.Payload, extracted.Artifacts)
}

func (r *DefaultOrchestrator) expandExternalizedPayload(
	ctx context.Context,
	logicalPath string,
	md metadata.ResourceMetadata,
	value resource.Value,
) (resource.Value, error) {
	entries, err := metadata.ResolveExternalizedAttributes(md)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return resource.Normalize(value)
	}

	var artifactStore repository.ResourceArtifactStore
	if r != nil && r.repository != nil {
		store, ok := r.repository.(repository.ResourceArtifactStore)
		if ok {
			artifactStore = store
		}
	}

	return resourceexternalization.Expand(ctx, artifactStore, logicalPath, value, entries)
}
