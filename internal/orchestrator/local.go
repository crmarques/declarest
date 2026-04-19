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
	"fmt"
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

func (r *Orchestrator) resolveLocalResourceForRead(
	ctx context.Context,
	logicalPath string,
) (resource.Resource, error) {
	manager, err := r.requireRepository()
	if err != nil {
		return resource.Resource{}, err
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	content, err := manager.Get(ctx, normalizedPath)
	if err == nil {
		resolvedMetadata, metadataErr := r.resolveMetadataForPath(ctx, normalizedPath, true)
		if metadataErr != nil {
			return resource.Resource{}, metadataErr
		}
		content, _, metadataErr = mergeContentWithMetadataDefaults(content, resolvedMetadata)
		if metadataErr != nil {
			return resource.Resource{}, metadataErr
		}
		normalizedValue, normalizeErr := resource.Normalize(content.Value)
		if normalizeErr != nil {
			return resource.Resource{}, normalizeErr
		}
		return resource.Resource{
			LogicalPath:       normalizedPath,
			CollectionPath:    collectionPathFor(normalizedPath),
			LocalAlias:        logicalPathAlias(normalizedPath),
			RemoteID:          logicalPathAlias(normalizedPath),
			Payload:           normalizedValue,
			PayloadDescriptor: content.Descriptor,
		}, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return resource.Resource{}, err
	}

	requestedInfo, requestedMd, infoErr := r.buildResourceInfoForRemoteRead(ctx, normalizedPath)
	if infoErr != nil {
		return resource.Resource{}, infoErr
	}
	if strings.TrimSpace(requestedMd.ID) == "" &&
		strings.TrimSpace(requestedMd.Alias) == "" {
		return resource.Resource{}, err
	}

	candidates, listErr := manager.List(ctx, requestedInfo.CollectionPath, repository.ListPolicy{})
	if listErr != nil {
		return resource.Resource{}, listErr
	}

	requestedSegment := requestedFallbackSegment(requestedInfo)
	if requestedSegment != "" {
		aliasMatches := make([]resource.Resource, 0, len(candidates))
		for _, candidate := range candidates {
			if listedResourceAlias(candidate) != requestedSegment {
				continue
			}
			aliasMatches = append(aliasMatches, candidate)
		}
		if len(aliasMatches) == 1 {
			hydrated, hydrateErr := r.hydrateLocalFallbackCandidate(ctx, manager, requestedMd, aliasMatches[0])
			if hydrateErr != nil {
				return resource.Resource{}, hydrateErr
			}
			return hydrated, nil
		}
	}

	matches := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		hydrated, hydrateErr := r.hydrateLocalFallbackCandidate(ctx, manager, requestedMd, candidate)
		if hydrateErr != nil {
			return resource.Resource{}, hydrateErr
		}
		if !matchesFallbackIdentity(
			requestedInfo,
			hydrated.LocalAlias,
			hydrated.RemoteID,
			hydrated.Payload,
		) {
			continue
		}

		matches = append(matches, hydrated)
	}

	switch len(matches) {
	case 0:
		return resource.Resource{}, err
	case 1:
		return matches[0], nil
	default:
		return resource.Resource{}, faults.NewTypedError(
			faults.ConflictError,
			fmt.Sprintf("local fallback for %q is ambiguous", normalizedPath),
			nil,
		)
	}
}

func (r *Orchestrator) hydrateLocalFallbackCandidate(
	ctx context.Context,
	manager repository.ResourceStore,
	md metadata.ResourceMetadata,
	candidate resource.Resource,
) (resource.Resource, error) {
	candidateContent, getErr := manager.Get(ctx, candidate.LogicalPath)
	if getErr != nil {
		return resource.Resource{}, getErr
	}
	candidateContent, _, defaultsErr := mergeContentWithMetadataDefaults(candidateContent, md)
	if defaultsErr != nil {
		return resource.Resource{}, defaultsErr
	}

	candidateContent, expandErr := r.expandExternalizedPayload(ctx, candidate.LogicalPath, md, candidateContent)
	if expandErr != nil {
		return resource.Resource{}, expandErr
	}

	candidatePayload, normalizeErr := resource.Normalize(candidateContent.Value)
	if normalizeErr != nil {
		return resource.Resource{}, normalizeErr
	}

	candidateAlias, resolvedRemoteID, identityErr := resolveResourceIdentity(
		candidate.LogicalPath,
		md,
		candidatePayload,
	)
	if identityErr != nil {
		return resource.Resource{}, identityErr
	}

	return resource.Resource{
		LogicalPath:       candidate.LogicalPath,
		CollectionPath:    collectionPathFor(candidate.LogicalPath),
		LocalAlias:        candidateAlias,
		RemoteID:          resolvedRemoteID,
		Payload:           candidatePayload,
		PayloadDescriptor: candidateContent.Descriptor,
	}, nil
}

func identityAttributeCandidates() []string {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 10)

	addCandidate := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		candidates = append(candidates, trimmed)
	}

	for _, fallback := range []string{
		resource.JSONPointerForObjectKey("id"),
		resource.JSONPointerForObjectKey("clientId"),
		resource.JSONPointerForObjectKey("name"),
		resource.JSONPointerForObjectKey("alias"),
		resource.JSONPointerForObjectKey("key"),
		resource.JSONPointerForObjectKey("uuid"),
		resource.JSONPointerForObjectKey("uid"),
	} {
		addCandidate(fallback)
	}

	return candidates
}

func resolveResourceIdentity(
	logicalPath string,
	md metadata.ResourceMetadata,
	value resource.Value,
) (string, string, error) {
	alias, remoteID, err := identity.ResolveAliasAndRemoteID(logicalPath, md, value)
	if err != nil {
		return "", "", err
	}

	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = logicalPathAlias(logicalPath)
	}

	remoteID = strings.TrimSpace(remoteID)
	if remoteID == "" {
		remoteID = alias
	}

	return alias, remoteID, nil
}

func resolvedRemoteIDFromPayload(md metadata.ResourceMetadata, value resource.Value) (string, bool) {
	if strings.TrimSpace(md.ID) == "" {
		return "", false
	}

	_, remoteID, err := identity.ResolveAliasAndRemoteID("/", md, value)
	if err != nil {
		return "", false
	}
	remoteID = strings.TrimSpace(remoteID)
	return remoteID, remoteID != ""
}

func logicalPathAlias(logicalPath string) string {
	if strings.TrimSpace(logicalPath) == "/" {
		return "/"
	}
	return path.Base(logicalPath)
}
