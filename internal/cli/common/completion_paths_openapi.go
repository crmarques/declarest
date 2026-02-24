package common

import (
	"context"
	"sort"
	"strings"

	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

type openAPIPathEntry struct {
	Path    string
	Methods map[string]struct{}
}

func addOpenAPISuggestions(
	suggestions map[string]struct{},
	entries []openAPIPathEntry,
	normalizedPrefix string,
	allowedMethods map[string]struct{},
) {
	for _, entry := range entries {
		if !openAPIPathEntryMatchesAllowedMethods(entry, allowedMethods) {
			continue
		}
		normalizedPathKey := normalizePathSuggestion(entry.Path)
		if normalizedPathKey == "" {
			continue
		}
		if normalizedPrefix != "" && !candidateRelevantForExpansion(normalizedPathKey, normalizedPrefix) {
			continue
		}
		addPathSuggestion(suggestions, entry.Path)
	}
}

func addSmartOpenAPISuggestions(
	ctx context.Context,
	orchestratorService orchestratordomain.CompletionService,
	suggestions map[string]struct{},
	localSeed []resource.Resource,
	remoteSeed []resource.Resource,
	entries []openAPIPathEntry,
	toComplete string,
	sourceStrategy completionSourceStrategy,
	allowedMethods map[string]struct{},
) {
	normalizedPrefix := normalizeCompletionPrefix(toComplete)
	resolver := newCollectionSegmentResolver(
		ctx,
		orchestratorService,
		localSeed,
		remoteSeed,
		maxTemplateQueries,
		sourceStrategy,
	)
	for _, entry := range entries {
		if !openAPIPathEntryMatchesAllowedMethods(entry, allowedMethods) {
			continue
		}

		normalizedTemplate := normalizePathSuggestion(entry.Path)
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
	orchestratorService orchestratordomain.CompletionService
	localSeed           []resource.Resource
	remoteSeed          []resource.Resource
	sourceStrategy      completionSourceStrategy
	cache               map[string][]string
	queryBudget         int
}

func newCollectionSegmentResolver(
	ctx context.Context,
	orchestratorService orchestratordomain.CompletionService,
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

func parseOpenAPIPathEntries(openAPISpec resource.Value) []openAPIPathEntry {
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

	entries := make([]openAPIPathEntry, 0, len(pathsMap))
	for pathKey, operations := range pathsMap {
		entries = append(entries, openAPIPathEntry{
			Path:    pathKey,
			Methods: normalizeOpenAPIMethods(operations),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func normalizeOpenAPIMethods(value any) map[string]struct{} {
	operations, ok := asStringMap(value)
	if !ok {
		return nil
	}

	methods := make(map[string]struct{}, len(operations))
	for method := range operations {
		clean := strings.ToLower(strings.TrimSpace(method))
		if clean == "" {
			continue
		}
		methods[clean] = struct{}{}
	}
	return methods
}

func openAPIPathEntryMatchesAllowedMethods(entry openAPIPathEntry, allowed map[string]struct{}) bool {
	if allowed == nil || len(allowed) == 0 {
		return true
	}
	if len(entry.Methods) == 0 {
		return true
	}
	for method := range entry.Methods {
		if _, ok := allowed[method]; ok {
			return true
		}
	}
	return false
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
