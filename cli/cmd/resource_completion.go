package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"declarest/internal/openapi"
	"declarest/internal/reconciler"
	"declarest/internal/resource"

	"github.com/spf13/cobra"
)

type pathCompletionSource int

const (
	pathCompletionSourceRepo pathCompletionSource = iota
	pathCompletionSourceRemote
)

type pathCompletionStrategy func(*cobra.Command) []pathCompletionSource

var resourcePathCompletionLoader = loadDefaultReconcilerSkippingRepoSync

type pathCompletionEntry struct {
	value       string
	description string
}

func (entry pathCompletionEntry) suggestion() string {
	if entry.description == "" {
		return entry.value
	}
	return entry.value + "\t" + entry.description
}

func completionEntriesToSortedList(entries map[string]pathCompletionEntry) []pathCompletionEntry {
	if len(entries) == 0 {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]pathCompletionEntry, 0, len(keys))
	for _, key := range keys {
		result = append(result, entries[key])
	}
	return result
}

func registerResourcePathCompletion(cmd *cobra.Command, strategy pathCompletionStrategy) {
	validator := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		return completeResourcePathCandidates(cmd, toComplete, strategy)
	}
	cmd.ValidArgsFunction = validator

	if cmd.Flag("path") != nil {
		_ = cmd.RegisterFlagCompletionFunc("path", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeResourcePathCandidates(cmd, toComplete, strategy)
		})
	}
}

func completeResourcePathCandidates(cmd *cobra.Command, toComplete string, strategy pathCompletionStrategy) ([]string, cobra.ShellCompDirective) {
	if strategy == nil {
		strategy = func(_ *cobra.Command) []pathCompletionSource {
			return []pathCompletionSource{pathCompletionSourceRepo}
		}
	}
	sources := strategy(cmd)
	if len(sources) == 0 {
		sources = []pathCompletionSource{pathCompletionSourceRepo}
	}

	recon, cleanup, err := resourcePathCompletionLoader()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	trimmed := strings.TrimSpace(toComplete)
	entries, hasOpenAPIChildren := gatherPathSuggestions(recon, trimmed, sources)
	if len(entries) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.suggestion())
	}
	directive := cobra.ShellCompDirectiveNoFileComp
	if hasOpenAPIChildren {
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return values, directive
}

func gatherPathSuggestions(recon *reconciler.DefaultReconciler, prefix string, sources []pathCompletionSource) ([]pathCompletionEntry, bool) {
	entries := make(map[string]pathCompletionEntry)
	for _, source := range sources {
		switch source {
		case pathCompletionSourceRepo:
			for _, candidate := range repoPathSuggestions(recon, prefix) {
				entries[candidate] = pathCompletionEntry{value: candidate}
			}
		case pathCompletionSourceRemote:
			for _, entry := range remotePathSuggestions(recon, prefix) {
				entries[entry.value] = entry
			}
		}
	}

	ioEntries := openAPIChildEntries(recon.ResourceRecordProvider, prefix)
	hasChildren := len(ioEntries) > 0
	for _, entry := range ioEntries {
		if _, exists := entries[entry.value]; exists {
			continue
		}
		entries[entry.value] = entry
	}

	return completionEntriesToSortedList(entries), hasChildren
}

func repoPathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []string {
	return filterPathsByPrefix(recon.RepositoryResourcePaths(), prefix)
}

func remotePathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []pathCompletionEntry {
	collection := remoteCompletionCollection(prefix)
	items, err := recon.ListRemoteResourceEntries(collection)
	if err != nil {
		return nil
	}
	return filterEntriesByPrefix(items, prefix)
}

func filterPathsByPrefix(paths []string, prefix string) []string {
	normalizedPrefix := normalizedCompletionPrefix(prefix)
	if normalizedPrefix == "" && strings.TrimSpace(prefix) == "" {
		normalizedPrefix = ""
	}
	var matches []string
	for _, candidate := range paths {
		candidate = resource.NormalizePath(candidate)
		if matchesCompletionPrefix(candidate, normalizedPrefix) {
			matches = append(matches, candidate)
		}
	}
	sort.Strings(matches)
	return matches
}

func filterEntriesByPrefix(entries []reconciler.RemoteResourceEntry, prefix string) []pathCompletionEntry {
	normalizedPrefix := normalizedCompletionPrefix(prefix)
	if normalizedPrefix == "" && strings.TrimSpace(prefix) == "" {
		normalizedPrefix = ""
	}
	matches := make(map[string]pathCompletionEntry)
	for _, item := range entries {
		remotePath := resource.NormalizePath(item.Path)
		aliasPath := resource.NormalizePath(item.AliasPath)
		if aliasPath == "" {
			aliasPath = remotePath
		}
		if remotePath == "" {
			continue
		}
		matchAlias := matchesCompletionPrefix(aliasPath, normalizedPrefix)
		matchRemote := matchesCompletionPrefix(remotePath, normalizedPrefix)
		if !matchAlias && !matchRemote {
			continue
		}
		displayPath := aliasPath
		if matchRemote && !matchAlias {
			displayPath = remotePath
		}
		if displayPath == "" {
			displayPath = remotePath
		}
		matches[displayPath] = pathCompletionEntry{
			value:       displayPath,
			description: completionDescription(item, displayPath),
		}
	}

	if len(matches) == 0 {
		return nil
	}
	keys := make([]string, 0, len(matches))
	for key := range matches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	results := make([]pathCompletionEntry, 0, len(keys))
	for _, key := range keys {
		results = append(results, matches[key])
	}
	return results
}

func cleanupCompletionAttribute(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}

func completionDescription(entry reconciler.RemoteResourceEntry, displayPath string) string {
	alias := cleanupCompletionAttribute(entry.Alias)
	if alias == "" {
		alias = cleanupCompletionAttribute(resource.LastSegment(entry.AliasPath))
	}
	if alias == "" {
		alias = cleanupCompletionAttribute(resource.LastSegment(displayPath))
	}
	id := cleanupCompletionAttribute(entry.ID)
	if alias == "" {
		return ""
	}
	if id == "" || id == alias {
		return ""
	}
	displaySegment := cleanupCompletionAttribute(resource.LastSegment(displayPath))
	switch displaySegment {
	case alias:
		return fmt.Sprintf("(%s)", id)
	case id:
		return alias
	default:
		return fmt.Sprintf("%s (%s)", alias, id)
	}
}

func matchesCompletionPrefix(candidate, prefix string) bool {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return prefix == ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.HasPrefix(candidate, trimmed)
}

func normalizedCompletionPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

func remoteCompletionCollection(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "/"
	}
	normalized := resource.NormalizePath(trimmed)
	if strings.HasSuffix(trimmed, "/") {
		return normalized
	}
	if normalized == "/" {
		return "/"
	}
	idx := strings.LastIndex(normalized, "/")
	if idx <= 0 {
		return "/"
	}
	return normalized[:idx]
}

func openAPIChildEntries(provider interface{}, prefix string) []pathCompletionEntry {
	spec := openapiSpecFromProvider(provider)
	if spec == nil {
		return nil
	}
	return specChildEntries(spec, prefix)
}

func specChildEntries(spec *openapi.Spec, prefix string) []pathCompletionEntry {
	type childInfo struct {
		path    string
		hasMore bool
	}

	normalizedPrefix := normalizedCompletionPrefix(prefix)
	prefixSegments := resource.SplitPathSegments(normalizedPrefix)
	trimmedPrefix := strings.TrimSpace(prefix)
	hasTrailingSlash := strings.HasSuffix(trimmedPrefix, "/")
	baseSegments := prefixSegments
	partialSegment := ""
	if !hasTrailingSlash && len(prefixSegments) > 0 {
		partialSegment = prefixSegments[len(prefixSegments)-1]
		baseSegments = prefixSegments[:len(prefixSegments)-1]
	}

	children := make(map[string]*childInfo)
	for _, item := range spec.Paths {
		if item == nil || item.Template == "" {
			continue
		}
		template := resource.NormalizePath(item.Template)
		if containsWildcardSegment(template) {
			continue
		}
		templateSegments := resource.SplitPathSegments(template)
		if len(templateSegments) <= len(baseSegments) {
			continue
		}
		if !segmentsMatchBase(templateSegments, baseSegments) {
			continue
		}
		nextSegment := templateSegments[len(baseSegments)]
		if nextSegment == "" || isOpenAPIPathParameter(nextSegment) {
			continue
		}
		if partialSegment != "" && !strings.HasPrefix(nextSegment, partialSegment) {
			continue
		}
		childSegments := append([]string{}, baseSegments...)
		childSegments = append(childSegments, nextSegment)
		childPath := "/" + strings.Join(childSegments, "/")
		childPath = resource.NormalizePath(childPath)

		info, exists := children[childPath]
		if !exists {
			info = &childInfo{path: childPath}
			children[childPath] = info
		}
		if len(templateSegments) > len(baseSegments)+1 {
			info.hasMore = true
		}
	}

	if len(children) == 0 {
		return nil
	}

	keys := make([]string, 0, len(children))
	for path := range children {
		keys = append(keys, path)
	}
	sort.Strings(keys)

	entries := make([]pathCompletionEntry, 0, len(keys))
	for _, key := range keys {
		info := children[key]
		value := key
		if info.hasMore {
			value = strings.TrimRight(key, "/") + "/"
		}
		entries = append(entries, pathCompletionEntry{value: value})
	}

	return entries
}

func segmentsMatchBase(segments, base []string) bool {
	if len(base) == 0 {
		return true
	}
	if len(base) > len(segments) {
		return false
	}
	for idx, part := range base {
		templ := segments[idx]
		if isOpenAPIPathParameter(templ) {
			continue
		}
		if templ != part {
			return false
		}
	}
	return true
}

func containsWildcardSegment(path string) bool {
	for _, segment := range resource.SplitPathSegments(path) {
		if segment == "_" {
			return true
		}
	}
	return false
}

func boolFlagValue(cmd *cobra.Command, name string, fallback bool) bool {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return fallback
	}
	value, err := strconv.ParseBool(flag.Value.String())
	if err != nil {
		return fallback
	}
	return value
}

func resourceGetPathStrategy(cmd *cobra.Command) []pathCompletionSource {
	if boolFlagValue(cmd, "repo", false) {
		return []pathCompletionSource{pathCompletionSourceRepo}
	}
	return []pathCompletionSource{pathCompletionSourceRemote}
}

func resourceDeletePathStrategy(cmd *cobra.Command) []pathCompletionSource {
	repoValue := boolFlagValue(cmd, "repo", true)
	remoteValue := boolFlagValue(cmd, "remote", false)
	repoChanged := cmd.Flags().Changed("repo")
	if remoteValue && repoValue && !repoChanged {
		repoValue = false
	}
	sources := make([]pathCompletionSource, 0, 2)
	if repoValue {
		sources = append(sources, pathCompletionSourceRepo)
	}
	if remoteValue {
		sources = append(sources, pathCompletionSourceRemote)
	}
	if len(sources) == 0 {
		return []pathCompletionSource{pathCompletionSourceRepo}
	}
	return sources
}

func resourceListPathStrategy(cmd *cobra.Command) []pathCompletionSource {
	repoValue := boolFlagValue(cmd, "repo", true)
	remoteValue := boolFlagValue(cmd, "remote", false)
	repoChanged := cmd.Flags().Changed("repo")
	if remoteValue && repoValue && !repoChanged {
		repoValue = false
	}
	sources := make([]pathCompletionSource, 0, 2)
	if repoValue {
		sources = append(sources, pathCompletionSourceRepo)
	}
	if remoteValue {
		sources = append(sources, pathCompletionSourceRemote)
	}
	if len(sources) == 0 {
		return []pathCompletionSource{pathCompletionSourceRepo}
	}
	return sources
}

func resourceRepoPathStrategy(*cobra.Command) []pathCompletionSource {
	return []pathCompletionSource{pathCompletionSourceRepo}
}

func resourceRemotePathStrategy(*cobra.Command) []pathCompletionSource {
	return []pathCompletionSource{pathCompletionSourceRemote}
}
