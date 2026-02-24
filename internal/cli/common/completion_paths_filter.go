package common

import (
	"path"
	"sort"
	"strings"
)

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
