package common

import (
	"context"
	"path"
	"sort"
	"strings"

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
	orchestratorService orchestratordomain.Orchestrator,
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

func addOpenAPISuggestions(
	suggestions map[string]struct{},
	openAPISpec resource.Value,
	normalizedPrefix string,
) {
	for _, pathKey := range openAPIPathKeys(openAPISpec) {
		normalizedPathKey := normalizePathSuggestion(pathKey)
		if normalizedPathKey == "" {
			continue
		}
		if normalizedPrefix != "" && !candidateRelevantForExpansion(normalizedPathKey, normalizedPrefix) {
			continue
		}
		addPathSuggestion(suggestions, pathKey)
	}
}

func addMetadataCollectionSuggestions(
	ctx context.Context,
	metadataService any,
	suggestions map[string]struct{},
	logicalPath string,
) {
	resolver, ok := metadataService.(metadataCollectionChildResolver)
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

func addSmartOpenAPISuggestions(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	suggestions map[string]struct{},
	localSeed []resource.Resource,
	remoteSeed []resource.Resource,
	openAPISpec resource.Value,
	toComplete string,
	sourceStrategy completionSourceStrategy,
) {
	templates := openAPIPathKeys(openAPISpec)
	if len(templates) == 0 {
		return
	}

	normalizedPrefix := normalizeCompletionPrefix(toComplete)
	resolver := newCollectionSegmentResolver(
		ctx,
		orchestratorService,
		localSeed,
		remoteSeed,
		maxTemplateQueries,
		sourceStrategy,
	)
	for _, templatePath := range templates {
		normalizedTemplate := normalizePathSuggestion(templatePath)
		if normalizedTemplate == "" || !containsTemplateSegments(normalizedTemplate) {
			continue
		}
		if !candidateRelevantForExpansion(normalizedTemplate, normalizedPrefix) {
			continue
		}

		for _, expandedPath := range expandTemplatePath(normalizedTemplate, normalizedPrefix, resolver) {
			addPathSuggestion(suggestions, expandedPath)
		}
	}
}

type collectionSegmentResolver struct {
	ctx                 context.Context
	orchestratorService orchestratordomain.Orchestrator
	localSeed           []resource.Resource
	remoteSeed          []resource.Resource
	sourceStrategy      completionSourceStrategy
	cache               map[string][]string
	queryBudget         int
}

type metadataCollectionChildResolver interface {
	ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error)
}

func newCollectionSegmentResolver(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	localSeed []resource.Resource,
	remoteSeed []resource.Resource,
	maxQueries int,
	sourceStrategy completionSourceStrategy,
) *collectionSegmentResolver {
	return &collectionSegmentResolver{
		ctx:                 ctx,
		orchestratorService: orchestratorService,
		localSeed:           localSeed,
		remoteSeed:          remoteSeed,
		sourceStrategy:      sourceStrategy,
		cache:               map[string][]string{},
		queryBudget:         maxQueries,
	}
}

func (r *collectionSegmentResolver) Resolve(collectionPath string) []string {
	normalizedCollectionPath := normalizePathSuggestion(collectionPath)
	if normalizedCollectionPath == "" || containsTemplateSegments(normalizedCollectionPath) {
		return nil
	}
	if cached, found := r.cache[normalizedCollectionPath]; found {
		return cached
	}

	primarySegments := map[string]struct{}{}
	addDirectChildSegmentsFromSource(
		primarySegments,
		normalizedCollectionPath,
		r.sourceStrategy.primary,
		r.localSeed,
		r.remoteSeed,
	)
	primaryItems, primaryErr := r.listCollectionChildren(normalizedCollectionPath, r.sourceStrategy.primary)
	if primaryErr == nil {
		addDirectChildSegmentsFromResources(primarySegments, normalizedCollectionPath, primaryItems)
	}

	segments := primarySegments
	if shouldQuerySecondarySource(
		r.sourceStrategy,
		sortedSetValues(primarySegments),
		"",
		primaryErr,
	) {
		addDirectChildSegmentsFromSource(
			segments,
			normalizedCollectionPath,
			r.sourceStrategy.secondary,
			r.localSeed,
			r.remoteSeed,
		)
		secondaryItems, secondaryErr := r.listCollectionChildren(
			normalizedCollectionPath,
			r.sourceStrategy.secondary,
		)
		if secondaryErr == nil {
			addDirectChildSegmentsFromResources(segments, normalizedCollectionPath, secondaryItems)
		}
	}

	resolved := sortedSetValues(segments)
	r.cache[normalizedCollectionPath] = resolved
	return resolved
}

func addDirectChildSegmentsFromSource(
	destination map[string]struct{},
	collectionPath string,
	source completionDataSource,
	localSeed []resource.Resource,
	remoteSeed []resource.Resource,
) {
	switch source {
	case completionSourceLocal:
		addDirectChildSegmentsFromResources(destination, collectionPath, localSeed)
	case completionSourceRemote:
		addDirectChildSegmentsFromResources(destination, collectionPath, remoteSeed)
	}
}

func (r *collectionSegmentResolver) listCollectionChildren(
	collectionPath string,
	source completionDataSource,
) ([]resource.Resource, error) {
	if source == completionSourceNone || r.queryBudget <= 0 {
		return nil, nil
	}
	r.queryBudget--

	return listCompletionResources(
		r.ctx,
		r.orchestratorService,
		source,
		collectionPath,
		false,
	)
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

func expandTemplatePath(
	templatePath string,
	normalizedPrefix string,
	resolver *collectionSegmentResolver,
) []string {
	segments := splitPathSegments(templatePath)
	if len(segments) == 0 {
		return nil
	}

	candidates := []string{"/"}
	for _, segment := range segments {
		nextCandidates := map[string]struct{}{}
		for _, candidate := range candidates {
			if isTemplateSegment(segment) {
				resolvedSegments := resolver.Resolve(candidate)
				if len(resolvedSegments) == 0 {
					placeholderPath := appendPathSegment(candidate, segment)
					if candidateRelevantForExpansion(placeholderPath, normalizedPrefix) {
						nextCandidates[placeholderPath] = struct{}{}
					}
					continue
				}

				for _, resolvedSegment := range resolvedSegments {
					resolvedPath := appendPathSegment(candidate, resolvedSegment)
					if candidateRelevantForExpansion(resolvedPath, normalizedPrefix) {
						nextCandidates[resolvedPath] = struct{}{}
					}
				}
			} else {
				resolvedPath := appendPathSegment(candidate, segment)
				if candidateRelevantForExpansion(resolvedPath, normalizedPrefix) {
					nextCandidates[resolvedPath] = struct{}{}
				}
			}
		}

		if len(nextCandidates) == 0 {
			return nil
		}
		candidates = sortedSetValuesLimited(nextCandidates, maxTemplateCandidates)
	}

	return candidates
}

func appendPathSegment(basePath string, segment string) string {
	trimmedSegment := strings.Trim(strings.TrimSpace(segment), "/")
	if trimmedSegment == "" {
		return normalizePathSuggestion(basePath)
	}
	if basePath == "/" || strings.TrimSpace(basePath) == "" {
		return normalizePathSuggestion("/" + trimmedSegment)
	}
	return normalizePathSuggestion(basePath + "/" + trimmedSegment)
}

func splitPathSegments(value string) []string {
	normalized := normalizePathSuggestion(value)
	if normalized == "" || normalized == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(normalized, "/"), "/")
}

func containsTemplateSegments(value string) bool {
	for _, segment := range splitPathSegments(value) {
		if isTemplateSegment(segment) {
			return true
		}
	}
	return false
}

func isTemplateSegment(segment string) bool {
	trimmed := strings.TrimSpace(segment)
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") && len(trimmed) > 2
}

func normalizeCompletionPrefix(value string) string {
	normalizedPrefix := strings.TrimSpace(value)
	if normalizedPrefix == "" {
		return ""
	}

	normalizedPrefix = unescapeCompletionToken(normalizedPrefix)
	if !strings.HasPrefix(normalizedPrefix, "/") {
		normalizedPrefix = "/" + strings.Trim(normalizedPrefix, "/")
	}
	return normalizedPrefix
}

func unescapeCompletionToken(value string) string {
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	escaped := false
	for _, item := range value {
		if escaped {
			builder.WriteRune(item)
			escaped = false
			continue
		}
		if item == '\\' {
			escaped = true
			continue
		}
		builder.WriteRune(item)
	}
	if escaped {
		builder.WriteRune('\\')
	}

	return builder.String()
}

func suggestionMatchesPrefix(suggestion string, normalizedPrefix string) bool {
	if normalizedPrefix == "" {
		return true
	}
	if strings.HasPrefix(suggestion, normalizedPrefix) {
		return true
	}
	if containsTemplateSegments(suggestion) {
		return templatePathMatchesPrefix(suggestion, normalizedPrefix)
	}
	return false
}

func candidateRelevantForExpansion(candidate string, normalizedPrefix string) bool {
	if normalizedPrefix == "" {
		return true
	}
	if suggestionMatchesPrefix(candidate, normalizedPrefix) {
		return true
	}
	return strings.HasPrefix(normalizedPrefix, candidate)
}

func templatePathMatchesPrefix(templatePath string, normalizedPrefix string) bool {
	if normalizedPrefix == "" || normalizedPrefix == "/" {
		return true
	}

	templateSegments := splitPathSegments(templatePath)
	prefixSegments, prefixEndsWithSlash := splitPrefixSegments(normalizedPrefix)
	if len(prefixSegments) == 0 {
		return true
	}
	if len(prefixSegments) > len(templateSegments) {
		return false
	}

	for idx, prefixSegment := range prefixSegments {
		templateSegment := templateSegments[idx]
		if isTemplateSegment(templateSegment) {
			continue
		}

		if idx == len(prefixSegments)-1 && !prefixEndsWithSlash {
			if !strings.HasPrefix(templateSegment, prefixSegment) {
				return false
			}
			continue
		}
		if templateSegment != prefixSegment {
			return false
		}
	}
	return true
}

func splitPrefixSegments(prefix string) ([]string, bool) {
	normalizedPrefix := normalizeCompletionPrefix(prefix)
	if normalizedPrefix == "" || normalizedPrefix == "/" {
		return nil, strings.HasSuffix(normalizedPrefix, "/")
	}

	endsWithSlash := strings.HasSuffix(normalizedPrefix, "/")
	trimmed := strings.TrimPrefix(normalizedPrefix, "/")
	if endsWithSlash {
		trimmed = strings.TrimSuffix(trimmed, "/")
	}
	if strings.TrimSpace(trimmed) == "" {
		return nil, endsWithSlash
	}
	return strings.Split(trimmed, "/"), endsWithSlash
}

func openAPIPathKeys(openAPISpec resource.Value) []string {
	root, ok := asStringMap(openAPISpec)
	if !ok {
		return nil
	}
	pathsValue, ok := root["paths"]
	if !ok {
		return nil
	}
	pathsMap, ok := asStringMap(pathsValue)
	if !ok {
		return nil
	}

	pathKeys := make([]string, 0, len(pathsMap))
	for pathKey := range pathsMap {
		pathKeys = append(pathKeys, pathKey)
	}
	sort.Strings(pathKeys)
	return pathKeys
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		mapped := make(map[string]any, len(typed))
		for key, item := range typed {
			stringKey, ok := key.(string)
			if !ok {
				return nil, false
			}
			mapped[stringKey] = item
		}
		return mapped, true
	default:
		return nil, false
	}
}

func addPathSuggestion(suggestions map[string]struct{}, value string) {
	normalized := normalizePathSuggestion(value)
	if normalized == "" {
		return
	}

	if normalized == "/" {
		suggestions["/"] = struct{}{}
		return
	}

	segments := strings.Split(strings.TrimPrefix(normalized, "/"), "/")
	current := ""
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		current += "/" + segment
		suggestions[current] = struct{}{}
	}
}

func normalizePathSuggestion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if trimmed == "/" {
		return "/"
	}

	cleaned := path.Clean("/" + strings.Trim(trimmed, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func filterPathSuggestions(suggestions map[string]struct{}, toComplete string) []string {
	normalizedPrefix := normalizeCompletionPrefix(toComplete)

	items := make([]string, 0, len(suggestions))
	for value := range suggestions {
		normalizedValue := normalizePathSuggestion(value)
		if normalizedValue == "" {
			continue
		}
		if !suggestionMatchesPrefix(normalizedValue, normalizedPrefix) {
			continue
		}
		items = append(items, normalizedValue)
	}

	scopedItems, scoped := restrictToNextLevelSuggestions(items, normalizedPrefix)
	if scoped {
		items = scopedItems
	} else {
		items = renderCollectionSuggestionsWithTrailingSlash(items)
	}
	items = removeTemplateSuggestions(items)
	sort.Strings(items)
	if len(items) > maxCompletionSuggestions {
		items = items[:maxCompletionSuggestions]
	}
	return items
}

type completionScope struct {
	parentPath     string
	partialSegment string
}

func resolveCompletionScope(normalizedPrefix string) (completionScope, bool) {
	trimmedPrefix := strings.TrimSpace(normalizedPrefix)
	if trimmedPrefix == "" {
		return completionScope{}, false
	}

	normalizedPath := normalizePathSuggestion(trimmedPrefix)
	if normalizedPath == "" {
		return completionScope{}, false
	}

	if strings.HasSuffix(trimmedPrefix, "/") {
		return completionScope{
			parentPath:     normalizedPath,
			partialSegment: "",
		}, true
	}

	parentPath := normalizePathSuggestion(path.Dir(normalizedPath))
	if parentPath == "" {
		parentPath = "/"
	}

	return completionScope{
		parentPath:     parentPath,
		partialSegment: strings.TrimSpace(path.Base(normalizedPath)),
	}, true
}

func restrictToNextLevelSuggestions(items []string, normalizedPrefix string) ([]string, bool) {
	scope, ok := resolveCompletionScope(normalizedPrefix)
	if !ok {
		return nil, false
	}

	type scopedSuggestion struct {
		hasDescendants bool
	}
	scoped := map[string]*scopedSuggestion{}

	for _, item := range items {
		normalizedItem := normalizePathSuggestion(item)
		if normalizedItem == "" {
			continue
		}

		childSegment, hasChild := firstChildSegment(scope.parentPath, normalizedItem)
		if !hasChild || strings.TrimSpace(childSegment) == "" || isTemplateSegment(childSegment) {
			continue
		}
		if scope.partialSegment != "" && !strings.HasPrefix(childSegment, scope.partialSegment) {
			continue
		}

		childPath := appendPathSegment(scope.parentPath, childSegment)
		entry, exists := scoped[childPath]
		if !exists {
			entry = &scopedSuggestion{}
			scoped[childPath] = entry
		}

		if normalizedItem != childPath && strings.HasPrefix(normalizedItem, childPath+"/") {
			entry.hasDescendants = true
		}
	}

	if len(scoped) == 0 {
		return nil, true
	}

	rendered := make(map[string]struct{}, len(scoped))
	for childPath, details := range scoped {
		if details.hasDescendants {
			rendered[childPath+"/"] = struct{}{}
			continue
		}
		rendered[childPath] = struct{}{}
	}

	return sortedSetValues(rendered), true
}

func renderCollectionSuggestionsWithTrailingSlash(items []string) []string {
	if len(items) == 0 {
		return items
	}

	normalizedItems := make([]string, 0, len(items))
	for _, item := range items {
		normalized := normalizePathSuggestion(item)
		if normalized == "" {
			continue
		}
		normalizedItems = append(normalizedItems, normalized)
	}

	collections := make(map[string]struct{})
	for _, parent := range normalizedItems {
		if parent == "/" {
			continue
		}
		parentPrefix := strings.TrimSuffix(parent, "/") + "/"
		for _, candidate := range normalizedItems {
			if candidate == parent {
				continue
			}
			if strings.HasPrefix(candidate, parentPrefix) {
				collections[parent] = struct{}{}
				break
			}
		}
	}

	rendered := make(map[string]struct{}, len(normalizedItems))
	for _, item := range normalizedItems {
		if _, collection := collections[item]; collection {
			rendered[item+"/"] = struct{}{}
			continue
		}
		rendered[item] = struct{}{}
	}

	return sortedSetValues(rendered)
}

func removeTemplateSuggestions(items []string) []string {
	if len(items) == 0 {
		return items
	}

	filtered := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		normalized := normalizePathSuggestion(trimmed)
		if normalized == "" || containsTemplateSegments(normalized) {
			continue
		}
		rendered := normalized
		if strings.HasSuffix(trimmed, "/") && normalized != "/" {
			rendered = normalized + "/"
		}
		if _, exists := seen[rendered]; exists {
			continue
		}
		seen[rendered] = struct{}{}
		filtered = append(filtered, rendered)
	}
	return filtered
}

func sortedSetValues(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func sortedSetValuesLimited(values map[string]struct{}, maxItems int) []string {
	items := sortedSetValues(values)
	if maxItems > 0 && len(items) > maxItems {
		items = items[:maxItems]
	}
	return items
}
