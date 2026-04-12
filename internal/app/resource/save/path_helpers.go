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

package save

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/app/resource/pathfallback"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

func normalizeSavePathPattern(rawPath string) (string, bool, bool, error) {
	parsedPath, err := resource.ParseRawPathWithOptions(rawPath, resource.RawPathParseOptions{})
	if err != nil {
		return "", false, false, err
	}

	hasWildcard := false
	for _, segment := range parsedPath.Segments {
		if segment == "_" {
			hasWildcard = true
			break
		}
	}

	return parsedPath.Normalized, hasWildcard, parsedPath.ExplicitCollectionTarget, nil
}

func resolveSaveRemoteValue(
	ctx context.Context,
	remoteReader orchestratordomain.RemoteReader,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	explicitCollectionTarget bool,
	skipItems []string,
) (resource.Content, error) {
	if explicitCollectionTarget {
		items, err := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
		if err == nil {
			items = resource.FilterCollectionItems(logicalPath, items, skipItems)
			return saveListPayloadFromResources(items), nil
		}
		if !isCollectionListShapeError(err) {
			return resource.Content{}, err
		}
	}

	remoteValue, err := remoteReader.GetRemote(ctx, logicalPath)
	if err == nil {
		return remoteValue, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return resource.Content{}, err
	}

	items, listErr := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
	if listErr != nil {
		return resource.Content{}, err
	}
	if !explicitCollectionTarget && !pathfallback.ShouldUseMetadataCollectionFallback(ctx, metadataService, logicalPath, items) {
		return resource.Content{}, err
	}
	items = resource.FilterCollectionItems(logicalPath, items, skipItems)

	return saveListPayloadFromResources(items), nil
}

func saveListPayloadFromResources(items []resource.Resource) resource.Content {
	if len(items) == 0 {
		return resource.Content{
			Value:      []any{},
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		}
	}

	sorted := make([]resource.Resource, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].LogicalPath < sorted[j].LogicalPath
	})

	payload := make([]any, 0, len(sorted))
	for _, item := range sorted {
		entry := map[string]any{
			"LogicalPath": item.LogicalPath,
			"Payload":     item.Payload,
		}
		if resource.IsPayloadDescriptorExplicit(item.PayloadDescriptor) {
			entry["PayloadDescriptor"] = map[string]any{
				"PayloadType": item.PayloadDescriptor.PayloadType,
				"MediaType":   item.PayloadDescriptor.MediaType,
				"Extension":   item.PayloadDescriptor.Extension,
			}
		}
		payload = append(payload, entry)
	}
	descriptor := sorted[0].PayloadDescriptor
	if !resource.IsPayloadDescriptorExplicit(descriptor) {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
	return resource.Content{
		Value:      payload,
		Descriptor: descriptor,
	}
}

func isCollectionListShapeError(err error) bool {
	return managedservicedomain.IsListPayloadShapeError(err)
}

func expandSaveWildcardPaths(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	wildcardPath string,
) ([]string, error) {
	segments := resource.SplitRawPathSegments(wildcardPath)
	if len(segments) == 0 {
		return nil, faults.NewValidationError("wildcard save path must target a collection or resource", nil)
	}

	currentPaths := []string{"/"}
	for _, segment := range segments {
		nextPaths := make(map[string]struct{})

		if segment == "_" {
			for _, parentPath := range currentPaths {
				items, err := orchestratorService.ListRemote(ctx, parentPath, orchestratordomain.ListPolicy{Recursive: false})
				if err != nil {
					return nil, err
				}

				for _, item := range items {
					childSegment, ok := resource.ChildSegment(parentPath, item.LogicalPath)
					if !ok {
						continue
					}
					childPath, err := appendSavePathSegment(parentPath, childSegment)
					if err != nil {
						return nil, err
					}
					nextPaths[childPath] = struct{}{}
				}
			}
		} else {
			for _, parentPath := range currentPaths {
				childPath, err := appendSavePathSegment(parentPath, segment)
				if err != nil {
					return nil, err
				}
				nextPaths[childPath] = struct{}{}
			}
		}

		if len(nextPaths) == 0 {
			return nil, faults.NewTypedError(
				faults.NotFoundError,
				fmt.Sprintf("no remote resources matched wildcard path %q", wildcardPath),
				nil,
			)
		}

		currentPaths = sortedPathKeys(nextPaths)
	}

	return currentPaths, nil
}

func appendSavePathSegment(parentPath string, segment string) (string, error) {
	trimmedSegment := strings.TrimSpace(segment)
	if trimmedSegment == "" {
		return "", faults.NewValidationError("wildcard path contains an empty segment", nil)
	}
	return resource.JoinLogicalPath(parentPath, trimmedSegment)
}

func sortedPathKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
