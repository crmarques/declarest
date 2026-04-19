// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/resourceexternalization"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func (r *Orchestrator) saveLocalResource(
	ctx context.Context,
	manager repository.ResourceStore,
	logicalPath string,
	content resource.Content,
) error {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return err
	}

	content = r.applyDefaultFormat(content, resolvedMetadata)

	entries, err := metadata.ResolveExternalizedAttributes(resolvedMetadata)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		content, _, err = compactContentWithMetadataDefaults(content, resolvedMetadata, true)
		if err != nil {
			return err
		}
		return manager.Save(ctx, normalizedPath, content)
	}

	artifactStore, ok := manager.(repository.ResourceArtifactStore)
	if !ok {
		return faults.NewTypedError(
			faults.InternalError,
			"repository store does not support resource artifacts",
			nil,
		)
	}

	extracted, err := resourceexternalization.Extract(content.Value, entries)
	if err != nil {
		return err
	}
	compactedContent, _, err := compactContentWithMetadataDefaults(
		resource.Content{
			Value:      extracted.Payload,
			Descriptor: content.Descriptor,
		},
		resolvedMetadata,
		true,
	)
	if err != nil {
		return err
	}

	return artifactStore.SaveResourceWithArtifacts(
		ctx,
		normalizedPath,
		compactedContent,
		extracted.Artifacts,
	)
}

func (r *Orchestrator) expandExternalizedPayload(
	ctx context.Context,
	logicalPath string,
	md metadata.ResourceMetadata,
	content resource.Content,
) (resource.Content, error) {
	entries, err := metadata.ResolveExternalizedAttributes(md)
	if err != nil {
		return resource.Content{}, err
	}
	if len(entries) == 0 {
		normalizedValue, normalizeErr := resource.Normalize(content.Value)
		if normalizeErr != nil {
			return resource.Content{}, normalizeErr
		}
		return resource.Content{
			Value:      normalizedValue,
			Descriptor: content.Descriptor,
		}, nil
	}

	var artifactStore repository.ResourceArtifactStore
	if r != nil && r.repository != nil {
		store, ok := r.repository.(repository.ResourceArtifactStore)
		if ok {
			artifactStore = store
		}
	}

	expanded, err := resourceexternalization.Expand(ctx, artifactStore, logicalPath, content.Value, entries)
	if err != nil {
		return resource.Content{}, err
	}
	return resource.Content{
		Value:      expanded,
		Descriptor: content.Descriptor,
	}, nil
}
