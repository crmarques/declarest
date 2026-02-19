package common

import (
	"context"
	"path"
	"sort"
	"strings"
	"time"

	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

const (
	completionTimeout        = 2 * time.Second
	maxCompletionSuggestions = 256
	maxTemplateQueries       = 32
	maxTemplateCandidates    = 128
)

var (
	outputCompletionValues = []string{
		OutputAuto,
		OutputText,
		OutputJSON,
		OutputYAML,
	}
	inputFormatCompletionValues = []string{
		OutputJSON,
		OutputYAML,
	}
)

func RegisterOutputFlagCompletion(command *cobra.Command) {
	RegisterFlagValueCompletions(command, "output", outputCompletionValues)
}

func RegisterInputFormatFlagCompletion(command *cobra.Command) {
	RegisterFlagValueCompletions(command, "format", inputFormatCompletionValues)
}

func RegisterFlagValueCompletions(command *cobra.Command, flagName string, values []string) {
	_ = command.RegisterFlagCompletionFunc(flagName, func(
		_ *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		return CompleteValues(values, toComplete)
	})
}

func RegisterContextFlagCompletion(command *cobra.Command, deps CommandDependencies) {
	_ = command.RegisterFlagCompletionFunc("context", func(
		_ *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		service, err := RequireContexts(deps)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		ctx, cancel := completionContext(context.Background())
		defer cancel()

		items, err := service.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, 0, len(items))
		for _, item := range items {
			names = append(names, item.Name)
		}
		return CompleteValues(names, toComplete)
	})
}

func RegisterPathFlagCompletion(command *cobra.Command, deps CommandDependencies) {
	_ = command.RegisterFlagCompletionFunc("path", func(
		command *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		return CompleteLogicalPaths(command, deps, toComplete)
	})
}

func SinglePathArgCompletionFunc(
	deps CommandDependencies,
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return CompleteLogicalPaths(command, deps, toComplete)
	}
}

func CompleteLogicalPaths(
	command *cobra.Command,
	deps CommandDependencies,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	suggestions := make(map[string]struct{})
	addPathSuggestion(suggestions, "/")

	orchestratorService, err := RequireOrchestrator(deps)
	if err != nil {
		return filterPathSuggestions(suggestions, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	ctx, cancel := completionContext(command.Context())
	defer cancel()

	localItems := []resource.Resource{}
	localItems, err = orchestratorService.ListLocal(ctx, "/", orchestratordomain.ListPolicy{Recursive: true})
	if err == nil {
		addResourceSuggestions(suggestions, localItems)
	}

	remoteItems := []resource.Resource{}
	remoteItems, err = orchestratorService.ListRemote(ctx, "/", orchestratordomain.ListPolicy{Recursive: true})
	if err == nil {
		addResourceSuggestions(suggestions, remoteItems)
	}

	openAPISpec, err := orchestratorService.GetOpenAPISpec(ctx)
	if err == nil {
		addOpenAPISuggestions(suggestions, openAPISpec)
		addSmartOpenAPISuggestions(ctx, orchestratorService, suggestions, localItems, remoteItems, openAPISpec, toComplete)
	}

	return filterPathSuggestions(suggestions, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func CompleteValues(values []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	trimmedPrefix := strings.TrimSpace(toComplete)
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		if trimmedPrefix != "" && !strings.HasPrefix(trimmedValue, trimmedPrefix) {
			continue
		}
		unique[trimmedValue] = struct{}{}
	}

	items := make([]string, 0, len(unique))
	for value := range unique {
		items = append(items, value)
	}
	sort.Strings(items)
	return items, cobra.ShellCompDirectiveNoFileComp
}

func completionContext(base context.Context) (context.Context, context.CancelFunc) {
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, completionTimeout)
}

func addResourceSuggestions(suggestions map[string]struct{}, items []resource.Resource) {
	for _, item := range items {
		addPathSuggestion(suggestions, item.CollectionPath)
		addPathSuggestion(suggestions, item.LogicalPath)
	}
}

func addOpenAPISuggestions(suggestions map[string]struct{}, openAPISpec resource.Value) {
	for _, pathKey := range openAPIPathKeys(openAPISpec) {
		addPathSuggestion(suggestions, pathKey)
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
) {
	templates := openAPIPathKeys(openAPISpec)
	if len(templates) == 0 {
		return
	}

	normalizedPrefix := normalizeCompletionPrefix(toComplete)
	resolver := newCollectionSegmentResolver(ctx, orchestratorService, localSeed, remoteSeed, maxTemplateQueries)
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
	cache               map[string][]string
	queryBudget         int
}

func newCollectionSegmentResolver(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	localSeed []resource.Resource,
	remoteSeed []resource.Resource,
	maxQueries int,
) *collectionSegmentResolver {
	return &collectionSegmentResolver{
		ctx:                 ctx,
		orchestratorService: orchestratorService,
		localSeed:           localSeed,
		remoteSeed:          remoteSeed,
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

	segments := map[string]struct{}{}
	addDirectChildSegmentsFromResources(segments, normalizedCollectionPath, r.localSeed)
	addDirectChildSegmentsFromResources(segments, normalizedCollectionPath, r.remoteSeed)

	if r.queryBudget > 0 {
		if localItems, err := r.orchestratorService.ListLocal(r.ctx, normalizedCollectionPath, orchestratordomain.ListPolicy{}); err == nil {
			addDirectChildSegmentsFromResources(segments, normalizedCollectionPath, localItems)
		}
		r.queryBudget--
	}

	if r.queryBudget > 0 {
		if remoteItems, err := r.orchestratorService.ListRemote(r.ctx, normalizedCollectionPath, orchestratordomain.ListPolicy{}); err == nil {
			addDirectChildSegmentsFromResources(segments, normalizedCollectionPath, remoteItems)
		}
		r.queryBudget--
	}

	resolved := sortedSetValues(segments)
	r.cache[normalizedCollectionPath] = resolved
	return resolved
}

func addDirectChildSegmentsFromResources(
	destination map[string]struct{},
	parentPath string,
	items []resource.Resource,
) {
	for _, item := range items {
		addDirectChildSegment(destination, parentPath, item.CollectionPath)
		addDirectChildSegment(destination, parentPath, item.LogicalPath)
	}
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

	normalizedPrefix = strings.ReplaceAll(normalizedPrefix, "\\", "/")
	if !strings.HasPrefix(normalizedPrefix, "/") {
		normalizedPrefix = "/" + strings.Trim(normalizedPrefix, "/")
	}
	return normalizedPrefix
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
		if !suggestionMatchesPrefix(value, normalizedPrefix) {
			continue
		}
		items = append(items, value)
	}
	sort.Strings(items)
	if len(items) > maxCompletionSuggestions {
		items = items[:maxCompletionSuggestions]
	}
	return items
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
