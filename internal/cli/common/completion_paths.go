package common

import (
	"context"
	"path"
	"strings"

	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	identitysupport "github.com/crmarques/declarest/resource/identity"
)

func completionContext(base context.Context) (context.Context, context.CancelFunc) {
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, completionTimeout)
}

func completionQueryPath(normalizedPrefix string) string {
	scope, ok := resolveCompletionScope(normalizedPrefix)
	if !ok {
		return "/"
	}
	return scope.parentPath
}

func queryScopedCompletionResources(
	ctx context.Context,
	orchestratorService orchestratordomain.CompletionService,
	source completionDataSource,
	parentPath string,
	suggestions map[string]struct{},
) ([]resource.Resource, int, error) {
	directItems, directErr := listCompletionResources(
		ctx,
		orchestratorService,
		source,
		parentPath,
		false,
	)

	aggregated := make([]resource.Resource, 0, len(directItems))
	if directErr == nil {
		aggregated = append(aggregated, directItems...)
		addResourceSuggestions(suggestions, directItems)
	}

	candidateCount := directChildCandidateCount(parentPath, aggregated)
	if shouldRunScopedRecursiveFallback(parentPath, candidateCount, directErr) {
		recursiveItems, recursiveErr := listCompletionResources(
			ctx,
			orchestratorService,
			source,
			parentPath,
			true,
		)
		if recursiveErr == nil {
			aggregated = append(aggregated, recursiveItems...)
			addResourceSuggestions(suggestions, recursiveItems)
			candidateCount = directChildCandidateCount(parentPath, aggregated)
		}
	}

	return aggregated, candidateCount, directErr
}

func shouldRunScopedRecursiveFallback(
	parentPath string,
	candidateCount int,
	directErr error,
) bool {
	if parentPath == "/" {
		return false
	}
	if directErr != nil {
		return false
	}
	return candidateCount <= 1
}

func directChildCandidateCount(parentPath string, items []resource.Resource) int {
	children := map[string]struct{}{}
	addDirectChildSegmentsFromResources(children, parentPath, items)
	return len(children)
}

func shouldRunRootRecursiveFallback(
	suggestions map[string]struct{},
	toComplete string,
) bool {
	return len(filterPathSuggestions(suggestions, toComplete)) == 0
}

func shouldRunSmartOpenAPISuggestions(
	suggestions map[string]struct{},
	toComplete string,
) bool {
	return len(filterPathSuggestions(suggestions, toComplete)) == 0
}

func addResourceSuggestions(suggestions map[string]struct{}, items []resource.Resource) {
	for _, item := range items {
		addPathSuggestion(suggestions, item.CollectionPath)
		addPathSuggestion(suggestions, completionResourcePath(item))
	}
}

func addMetadataCollectionSuggestions(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	suggestions map[string]struct{},
	logicalPath string,
) {
	resolver, ok := metadataService.(metadatadomain.CollectionChildrenResolver)
	if !ok {
		return
	}

	normalizedPath := normalizePathSuggestion(logicalPath)
	if normalizedPath == "" {
		normalizedPath = "/"
	}

	children, err := resolver.ResolveCollectionChildren(ctx, normalizedPath)
	if err != nil {
		return
	}
	for _, child := range children {
		segment := strings.TrimSpace(child)
		if segment == "" || segment == "_" || containsTemplateSegments(segment) {
			continue
		}
		addPathSuggestion(suggestions, appendPathSegment(normalizedPath, segment))
	}
}

func addDirectChildSegmentsFromResources(
	destination map[string]struct{},
	parentPath string,
	items []resource.Resource,
) {
	for _, item := range items {
		addDirectChildSegment(destination, parentPath, item.CollectionPath)
		addDirectChildSegment(destination, parentPath, completionResourcePath(item))
	}
}

func completionResourcePath(item resource.Resource) string {
	normalizedLogicalPath := normalizePathSuggestion(item.LogicalPath)
	if normalizedLogicalPath == "" || normalizedLogicalPath == "/" {
		return normalizedLogicalPath
	}

	collectionPath := normalizePathSuggestion(item.CollectionPath)
	if collectionPath == "" {
		collectionPath = normalizePathSuggestion(path.Dir(normalizedLogicalPath))
	}

	aliasSegment := completionAliasSegment(item)
	if aliasSegment == "" || aliasSegment == "_" || strings.Contains(aliasSegment, "/") {
		return normalizedLogicalPath
	}

	if collectionPath == "" || collectionPath == "/" {
		return normalizePathSuggestion("/" + aliasSegment)
	}

	parentSegment := strings.TrimSpace(path.Base(collectionPath))
	if parentSegment != "" && aliasSegment == parentSegment {
		candidatePath := appendPathSegment(collectionPath, aliasSegment)
		// Avoid self-loop suggestions like ".../AD PRD/AD PRD". These can be
		// emitted by provider list responses for alias-resolved paths and do not
		// advance completion to the next level.
		if normalizedLogicalPath == candidatePath {
			return collectionPath
		}
		// If logical path doesn't live under this collection, prefer logical path
		// and let scoped filtering/fallback handle it.
		if !strings.HasPrefix(normalizedLogicalPath, strings.TrimSuffix(collectionPath, "/")+"/") {
			return normalizedLogicalPath
		}
	}

	// Some list providers can return collection-like entries where the
	// collection path equals the logical path. In that case, appending alias
	// would duplicate the last segment (for example ".../AD PRD/AD PRD").
	if normalizedLogicalPath == collectionPath {
		return normalizedLogicalPath
	}
	return appendPathSegment(collectionPath, aliasSegment)
}

func completionAliasSegment(item resource.Resource) string {
	if payloadMap, ok := item.Payload.(map[string]any); ok {
		if aliasAttribute := strings.TrimSpace(item.Metadata.AliasFromAttribute); aliasAttribute != "" {
			if value, found := identitysupport.LookupScalarAttribute(payloadMap, aliasAttribute); found {
				trimmedValue := strings.TrimSpace(value)
				if trimmedValue != "" {
					return trimmedValue
				}
			}
		}
	}

	trimmedAlias := strings.TrimSpace(item.LocalAlias)
	if trimmedAlias != "" && trimmedAlias != "/" {
		return trimmedAlias
	}

	return strings.TrimSpace(path.Base(item.LogicalPath))
}

func addDirectChildSegment(destination map[string]struct{}, parentPath string, candidatePath string) {
	segment, ok := firstChildSegment(parentPath, candidatePath)
	if !ok || segment == "_" {
		return
	}
	destination[segment] = struct{}{}
}

func firstChildSegment(parentPath string, candidatePath string) (string, bool) {
	normalizedParent := normalizePathSuggestion(parentPath)
	normalizedCandidate := normalizePathSuggestion(candidatePath)
	if normalizedParent == "" || normalizedCandidate == "" {
		return "", false
	}
	if normalizedParent == normalizedCandidate {
		return "", false
	}

	if normalizedParent == "/" {
		remaining := strings.TrimPrefix(normalizedCandidate, "/")
		if remaining == "" {
			return "", false
		}
		segments := strings.SplitN(remaining, "/", 2)
		if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
			return "", false
		}
		return strings.TrimSpace(segments[0]), true
	}

	parentPrefix := strings.TrimSuffix(normalizedParent, "/")
	if !strings.HasPrefix(normalizedCandidate, parentPrefix+"/") {
		return "", false
	}
	remaining := strings.TrimPrefix(normalizedCandidate, parentPrefix+"/")
	if remaining == "" {
		return "", false
	}
	segments := strings.SplitN(remaining, "/", 2)
	if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
		return "", false
	}
	return strings.TrimSpace(segments[0]), true
}
