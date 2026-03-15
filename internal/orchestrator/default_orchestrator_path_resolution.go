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
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

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

func matchesFallbackIdentity(
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

	for _, attribute := range identityAttributeCandidates() {
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

func matchesFallbackCandidate(requestedInfo resource.Resource, candidate resource.Resource) bool {
	return matchesFallbackIdentity(
		requestedInfo,
		candidate.LocalAlias,
		candidate.RemoteID,
		candidate.Payload,
	)
}

func matchesResolvedIdentityCandidate(resolvedResource resource.Resource, candidate resource.Resource) bool {
	if candidate.LocalAlias == resolvedResource.LocalAlias {
		return true
	}
	return strings.TrimSpace(resolvedResource.RemoteID) != "" && candidate.RemoteID == resolvedResource.RemoteID
}

func fallbackSegmentValue(candidate resource.Resource) string {
	if value := strings.TrimSpace(candidate.RemoteID); value != "" {
		return value
	}

	payload, ok := candidate.Payload.(map[string]any)
	if ok {
		for _, attribute := range identityAttributeCandidates() {
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

func remoteReadResourceFromFallbackCandidate(
	requested resource.Resource,
	candidate resource.Resource,
) resource.Resource {
	resolved := requested
	resolved.LocalAlias = strings.TrimSpace(candidate.LocalAlias)
	if resolved.LocalAlias == "" {
		resolved.LocalAlias = logicalPathAlias(requested.LogicalPath)
	}
	resolved.RemoteID = strings.TrimSpace(candidate.RemoteID)
	if resolved.RemoteID == "" {
		resolved.RemoteID = fallbackSegmentValue(candidate)
	}
	if candidate.Payload != nil {
		resolved.Payload = resource.DeepCopyValue(candidate.Payload)
	}
	if resource.IsPayloadDescriptorExplicit(candidate.PayloadDescriptor) {
		resolved.PayloadDescriptor = candidate.PayloadDescriptor
	}
	return resolved
}

func remoteFallbackCandidates(
	requested resource.Resource,
	candidates []resource.Resource,
) ([]resource.Resource, error) {
	matched := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if matchesFallbackCandidate(requested, candidate) {
			matched = append(matched, candidate)
		}
	}

	if len(matched) <= 1 {
		return matched, nil
	}

	return nil, faults.NewTypedError(
		faults.ConflictError,
		fmt.Sprintf("remote fallback for %q is ambiguous", requested.LogicalPath),
		nil,
	)
}

func (r *Orchestrator) resolveRemoteCollectionCandidate(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	requested resource.Resource,
	md metadata.ResourceMetadata,
) (resource.Resource, bool, error) {
	candidates, listErr := r.listRemoteResources(ctx, serverManager, requested.CollectionPath, md)
	if listErr != nil {
		return resource.Resource{}, false, listErr
	}

	matched, matchErr := remoteFallbackCandidates(requested, candidates)
	if matchErr != nil {
		return resource.Resource{}, true, matchErr
	}
	if len(matched) == 0 {
		return resource.Resource{}, false, nil
	}

	return matched[0], true, nil
}

func (r *Orchestrator) fetchRemoteValueFromCollectionCandidate(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	requested resource.Resource,
	md metadata.ResourceMetadata,
) (resource.Content, bool, error) {
	candidate, handled, err := r.resolveRemoteCollectionCandidate(ctx, serverManager, requested, md)
	if !handled {
		return resource.Content{}, false, err
	}
	if err != nil {
		return resource.Content{}, true, err
	}

	return r.fetchRemoteValueForCandidate(ctx, serverManager, requested, md, candidate)
}

func (r *Orchestrator) fetchRemoteValueForCandidate(
	ctx context.Context,
	serverManager managedserver.ManagedServerClient,
	requested resource.Resource,
	md metadata.ResourceMetadata,
	candidate resource.Resource,
) (resource.Content, bool, error) {
	resolvedCandidate := remoteReadResourceFromFallbackCandidate(requested, candidate)
	value, getErr := serverManager.Get(ctx, resolvedCandidate, md)
	if getErr == nil {
		if ambiguityErr := r.detectRemoteIdentityAmbiguityAfterDirectGet(ctx, serverManager, resolvedCandidate, md); ambiguityErr != nil {
			return resource.Content{}, true, ambiguityErr
		}
		return value, true, nil
	}

	if faults.IsCategory(getErr, faults.NotFoundError) || faults.IsCategory(getErr, faults.ValidationError) {
		return contentFromResource(candidate), true, nil
	}

	return resource.Content{}, true, getErr
}

func canUseRemoteCollectionCandidateFallback(err error) bool {
	return faults.IsCategory(err, faults.NotFoundError) || faults.IsCategory(err, faults.ValidationError)
}
