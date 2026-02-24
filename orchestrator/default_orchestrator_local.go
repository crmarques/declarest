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

func (r *DefaultOrchestrator) resolveLocalResourceForRead(
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

	value, err := manager.Get(ctx, normalizedPath)
	if err == nil {
		normalizedValue, normalizeErr := resource.Normalize(value)
		if normalizeErr != nil {
			return resource.Resource{}, normalizeErr
		}
		return resource.Resource{
			LogicalPath:    normalizedPath,
			CollectionPath: collectionPathFor(normalizedPath),
			LocalAlias:     logicalPathAlias(normalizedPath),
			RemoteID:       logicalPathAlias(normalizedPath),
			Payload:        normalizedValue,
		}, nil
	}
	if !isTypedCategory(err, faults.NotFoundError) {
		return resource.Resource{}, err
	}

	requestedInfo, infoErr := r.buildResourceInfoForRemoteRead(ctx, normalizedPath)
	if infoErr != nil {
		return resource.Resource{}, infoErr
	}
	if strings.TrimSpace(requestedInfo.Metadata.IDFromAttribute) == "" &&
		strings.TrimSpace(requestedInfo.Metadata.AliasFromAttribute) == "" {
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
			hydrated, hydrateErr := r.hydrateLocalFallbackCandidate(ctx, manager, requestedInfo, aliasMatches[0])
			if hydrateErr != nil {
				return resource.Resource{}, hydrateErr
			}
			return hydrated, nil
		}
	}

	matches := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		hydrated, hydrateErr := r.hydrateLocalFallbackCandidate(ctx, manager, requestedInfo, candidate)
		if hydrateErr != nil {
			return resource.Resource{}, hydrateErr
		}
		if !matchesLocalFallbackIdentity(
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

func (r *DefaultOrchestrator) hydrateLocalFallbackCandidate(
	ctx context.Context,
	manager repository.ResourceStore,
	requestedInfo resource.Resource,
	candidate resource.Resource,
) (resource.Resource, error) {
	candidateValue, getErr := manager.Get(ctx, candidate.LogicalPath)
	if getErr != nil {
		return resource.Resource{}, getErr
	}

	candidatePayload, normalizeErr := resource.Normalize(candidateValue)
	if normalizeErr != nil {
		return resource.Resource{}, normalizeErr
	}

	candidateAlias, resolvedRemoteID, identityErr := resolveResourceIdentity(
		candidate.LogicalPath,
		requestedInfo.Metadata,
		candidatePayload,
	)
	if identityErr != nil {
		return resource.Resource{}, identityErr
	}

	return resource.Resource{
		LogicalPath:    candidate.LogicalPath,
		CollectionPath: collectionPathFor(candidate.LogicalPath),
		LocalAlias:     candidateAlias,
		RemoteID:       resolvedRemoteID,
		Metadata:       requestedInfo.Metadata,
		Payload:        candidatePayload,
	}, nil
}

func listedResourceAlias(item resource.Resource) string {
	alias := strings.TrimSpace(item.LocalAlias)
	if alias != "" && alias != "/" {
		return alias
	}
	return logicalPathAlias(item.LogicalPath)
}

func requestedFallbackSegment(requestedInfo resource.Resource) string {
	requestedSegment := strings.TrimSpace(requestedInfo.RemoteID)
	if requestedSegment == "" {
		requestedSegment = logicalPathAlias(requestedInfo.LogicalPath)
	}
	return requestedSegment
}

func matchesLocalFallbackIdentity(
	requestedInfo resource.Resource,
	candidateAlias string,
	candidateRemoteID string,
	candidatePayload resource.Value,
) bool {
	requestedSegment := requestedFallbackSegment(requestedInfo)
	if requestedSegment == "" {
		return false
	}

	if strings.TrimSpace(candidateRemoteID) == requestedSegment || strings.TrimSpace(candidateAlias) == requestedSegment {
		return true
	}

	payloadMap, ok := candidatePayload.(map[string]any)
	if !ok {
		return false
	}

	identityCandidates := identityAttributeCandidates(requestedInfo.Metadata)
	for _, attribute := range identityCandidates {
		value, found := identity.LookupScalarAttribute(payloadMap, attribute)
		if !found || strings.TrimSpace(value) == "" {
			continue
		}
		if strings.TrimSpace(value) == requestedSegment {
			return true
		}
	}

	return false
}

func identityAttributeCandidates(md metadata.ResourceMetadata) []string {
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

	addCandidate(md.IDFromAttribute)
	addCandidate(md.AliasFromAttribute)

	// Keep local fallback usable when repository metadata points at aliases only.
	for _, fallback := range []string{"id", "clientId", "name", "alias", "key", "uuid", "uid"} {
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

func logicalPathAlias(logicalPath string) string {
	if strings.TrimSpace(logicalPath) == "/" {
		return "/"
	}
	return path.Base(logicalPath)
}
