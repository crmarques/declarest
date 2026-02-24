package save

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/app/resource/pathfallback"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	serverdomain "github.com/crmarques/declarest/server"
)

func normalizeSavePathPattern(rawPath string) (string, bool, bool, error) {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return "", false, false, validationError("path is required", nil)
	}
	explicitCollectionTarget := trimmedPath != "/" && strings.HasSuffix(trimmedPath, "/")

	normalizedInput := strings.ReplaceAll(trimmedPath, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", false, false, validationError("logical path must be absolute", nil)
	}

	for _, segment := range strings.Split(normalizedInput, "/") {
		if segment == ".." {
			return "", false, false, validationError("logical path must not contain traversal segments", nil)
		}
	}

	normalizedPath := path.Clean(normalizedInput)
	if !strings.HasPrefix(normalizedPath, "/") {
		return "", false, false, validationError("logical path must be absolute", nil)
	}
	if normalizedPath != "/" {
		normalizedPath = strings.TrimSuffix(normalizedPath, "/")
	}

	hasWildcard := false
	for _, segment := range splitSavePathSegments(normalizedPath) {
		if segment == "_" {
			hasWildcard = true
			break
		}
	}

	return normalizedPath, hasWildcard, explicitCollectionTarget, nil
}

func resolveSaveRemoteValue(
	ctx context.Context,
	remoteReader saveRemoteReader,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	explicitCollectionTarget bool,
) (resource.Value, error) {
	if explicitCollectionTarget {
		items, err := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
		if err == nil {
			return saveListPayloadFromResources(items), nil
		}
		if !isCollectionListShapeError(err) {
			return nil, err
		}
	}

	remoteValue, err := remoteReader.GetRemote(ctx, logicalPath)
	if err == nil {
		return remoteValue, nil
	}
	if !isTypedErrorCategory(err, faults.NotFoundError) {
		return nil, err
	}

	items, listErr := remoteReader.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{})
	if listErr != nil {
		return nil, err
	}
	if !explicitCollectionTarget && !pathfallback.ShouldUseMetadataCollectionFallback(ctx, metadataService, logicalPath, items) {
		return nil, err
	}

	return saveListPayloadFromResources(items), nil
}

func saveListPayloadFromResources(items []resource.Resource) resource.Value {
	if len(items) == 0 {
		return []any{}
	}

	sorted := make([]resource.Resource, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i].LogicalPath < sorted[j].LogicalPath
	})

	payload := make([]any, 0, len(sorted))
	for _, item := range sorted {
		payload = append(payload, item.Payload)
	}
	return payload
}

func isCollectionListShapeError(err error) bool {
	return serverdomain.IsListPayloadShapeError(err)
}

func splitSavePathSegments(logicalPath string) []string {
	trimmed := strings.Trim(strings.TrimSpace(logicalPath), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func expandSaveWildcardPaths(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	wildcardPath string,
) ([]string, error) {
	segments := splitSavePathSegments(wildcardPath)
	if len(segments) == 0 {
		return nil, validationError("wildcard save path must target a collection or resource", nil)
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
					childSegment, ok := directChildSegment(parentPath, item.LogicalPath)
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
		return "", validationError("wildcard path contains an empty segment", nil)
	}
	return resource.JoinLogicalPath(parentPath, trimmedSegment)
}

func directChildSegment(parentPath string, candidatePath string) (string, bool) {
	return resource.ChildSegment(parentPath, candidatePath)
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
