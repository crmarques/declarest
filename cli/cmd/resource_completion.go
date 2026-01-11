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

type completionPrefixInfo struct {
	trimmed          string
	lastToken        string
	hasTrailingSlash bool
}

type openAPICompletionInfo struct {
	entries      []pathCompletionEntry
	hasEntries   bool
	hasSpec      bool
	hasPrefix    bool
	isCollection bool
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

func newCompletionPrefixInfo(prefix string) completionPrefixInfo {
	trimmed := strings.TrimSpace(prefix)
	hasTrailingSlash := strings.HasSuffix(trimmed, "/")
	lastToken := ""
	if trimmed != "" {
		if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
			lastToken = trimmed[idx+1:]
		} else {
			lastToken = trimmed
		}
	}
	if hasTrailingSlash {
		lastToken = ""
	}
	return completionPrefixInfo{
		trimmed:          trimmed,
		lastToken:        lastToken,
		hasTrailingSlash: hasTrailingSlash,
	}
}

func newOpenAPICompletionInfo(provider interface{}, prefix string) openAPICompletionInfo {
	spec := openapiSpecFromProvider(provider)
	if spec == nil {
		return openAPICompletionInfo{}
	}
	entries := specChildEntries(spec, prefix)
	return openAPICompletionInfo{
		entries:      entries,
		hasEntries:   len(entries) > 0,
		hasSpec:      true,
		hasPrefix:    specHasPrefix(spec, prefix),
		isCollection: specCollectionPath(spec, prefix),
	}
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
	info := newCompletionPrefixInfo(prefix)
	openAPIInfo := newOpenAPICompletionInfo(recon.ResourceRecordProvider, info.trimmed)
	if openAPIInfo.hasEntries && info.lastToken != "" {
		return openAPIInfo.entries, true
	}

	if info.hasTrailingSlash && openAPIInfo.hasSpec && openAPIInfo.hasPrefix && !openAPIInfo.isCollection {
		return openAPIInfo.entries, openAPIInfo.hasEntries
	}

	entries := make(map[string]pathCompletionEntry)
	if info.hasTrailingSlash && (openAPIInfo.isCollection || !openAPIInfo.hasSpec || !openAPIInfo.hasPrefix) {
		for _, source := range sources {
			switch source {
			case pathCompletionSourceRepo:
				for _, entry := range repoCollectionSuggestions(recon, prefix) {
					entries[entry.value] = entry
				}
			case pathCompletionSourceRemote:
				for _, entry := range remoteCollectionSuggestions(recon, prefix) {
					entries[entry.value] = entry
				}
			}
		}
	} else {
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
	}

	for _, entry := range openAPIInfo.entries {
		if _, exists := entries[entry.value]; exists {
			continue
		}
		entries[entry.value] = entry
	}

	return completionEntriesToSortedList(entries), openAPIInfo.hasEntries
}

func repoPathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []string {
	return filterPathsByPrefix(recon.RepositoryResourcePaths(), prefix)
}

func repoCollectionSuggestions(recon *reconciler.DefaultReconciler, prefix string) []pathCompletionEntry {
	if recon == nil {
		return nil
	}
	base := completionCollectionBase(prefix)
	if base == "" {
		return nil
	}
	baseSegments := resource.SplitPathSegments(base)
	baseDepth := len(baseSegments)
	prefixPath := base
	if prefixPath != "/" {
		prefixPath += "/"
	}

	info := collectionResourceInfo(recon, prefix)
	entries := make(map[string]pathCompletionEntry)
	for _, path := range recon.RepositoryResourcePaths() {
		normalized := resource.NormalizePath(path)
		if !strings.HasPrefix(normalized, prefixPath) {
			continue
		}
		if len(resource.SplitPathSegments(normalized)) != baseDepth+1 {
			continue
		}
		res, err := recon.GetLocalResource(normalized)
		if err != nil {
			res = resource.Resource{}
		}
		fallback := resource.LastSegment(normalized)
		idValue, aliasValue := collectionEntryValues(info, res, fallback)
		entry, ok := collectionCompletionEntry(base, idValue, idValue, aliasValue, info)
		if !ok {
			continue
		}
		entries[entry.value] = entry
	}

	return completionEntriesToSortedList(entries)
}

func remotePathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []pathCompletionEntry {
	collection := remoteCompletionCollection(prefix)
	items, err := recon.ListRemoteResourceEntries(collection)
	if err != nil {
		return nil
	}
	return filterEntriesByPrefix(items, prefix)
}

func remoteCollectionSuggestions(recon *reconciler.DefaultReconciler, prefix string) []pathCompletionEntry {
	if recon == nil {
		return nil
	}
	base := completionCollectionBase(prefix)
	if base == "" {
		return nil
	}
	info := collectionResourceInfo(recon, prefix)

	items, err := recon.ListRemoteResourceEntries(base)
	if err != nil {
		return nil
	}

	entries := make(map[string]pathCompletionEntry)
	for _, item := range items {
		idSegment := resource.LastSegment(item.Path)
		entry, ok := collectionCompletionEntry(base, idSegment, item.ID, item.Alias, info)
		if !ok {
			continue
		}
		entries[entry.value] = entry
	}

	return completionEntriesToSortedList(entries)
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

func completionCollectionBase(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "/"
	}
	return resource.NormalizePath(trimmed)
}

func collectionResourceInfo(recon *reconciler.DefaultReconciler, prefix string) *resource.ResourceInfoMetadata {
	if recon == nil {
		return nil
	}
	collectionPath := normalizedCompletionPrefix(prefix)
	if collectionPath == "" {
		return nil
	}
	if collectionPath != "/" && !strings.HasSuffix(collectionPath, "/") {
		collectionPath += "/"
	}
	meta, err := recon.ResourceMetadata(collectionPath)
	if err != nil {
		return nil
	}
	return meta.ResourceInfo
}

func collectionEntryValues(info *resource.ResourceInfoMetadata, res resource.Resource, fallback string) (string, string) {
	idAttr, aliasAttr := collectionAttributeNames(info)
	idValue := completionAttributeValue(res, idAttr, fallback)
	aliasValue := completionAttributeValue(res, aliasAttr, fallback)
	return idValue, aliasValue
}

func collectionAttributeNames(info *resource.ResourceInfoMetadata) (string, string) {
	if info == nil {
		return "", ""
	}
	return strings.TrimSpace(info.IDFromAttribute), strings.TrimSpace(info.AliasFromAttribute)
}

func completionAttributeValue(res resource.Resource, attr string, fallback string) string {
	attr = strings.TrimSpace(attr)
	if attr != "" {
		if value, ok := resource.LookupValueFromResource(res, attr); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func collectionCompletionEntry(basePath, idSegment, idValue, aliasValue string, info *resource.ResourceInfoMetadata) (pathCompletionEntry, bool) {
	segment := sanitizeCompletionSegment(idSegment)
	if segment == "" {
		segment = sanitizeCompletionSegment(idValue)
	}
	if segment == "" {
		return pathCompletionEntry{}, false
	}
	value := completionPathForSegment(basePath, segment)
	if value == "" {
		return pathCompletionEntry{}, false
	}
	description := collectionAliasDescription(info, idValue, aliasValue)
	return pathCompletionEntry{value: value, description: description}, true
}

func completionPathForSegment(basePath, segment string) string {
	base := resource.NormalizePath(basePath)
	segment = strings.Trim(segment, "/")
	if segment == "" {
		return ""
	}
	if base == "/" {
		return resource.NormalizePath("/" + segment)
	}
	return resource.NormalizePath(strings.TrimRight(base, "/") + "/" + segment)
}

func sanitizeCompletionSegment(segment string) string {
	segment = cleanupCompletionAttribute(segment)
	segment = strings.ReplaceAll(segment, "/", "-")
	segment = strings.ReplaceAll(segment, "\\", "-")
	return segment
}

func collectionAliasDescription(info *resource.ResourceInfoMetadata, idValue, aliasValue string) string {
	if completionUsesSameAttribute(info) {
		return ""
	}
	idValue = cleanupCompletionAttribute(idValue)
	aliasValue = cleanupCompletionAttribute(aliasValue)
	if aliasValue == "" || aliasValue == idValue {
		return ""
	}
	return aliasValue
}

func completionUsesSameAttribute(info *resource.ResourceInfoMetadata) bool {
	idAttr, aliasAttr := collectionAttributeNames(info)
	return idAttr != "" && aliasAttr != "" && idAttr == aliasAttr
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
		return id
	case id:
		return alias
	default:
		return fmt.Sprintf("%s %s", alias, id)
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

func specHasPrefix(spec *openapi.Spec, prefix string) bool {
	if spec == nil {
		return false
	}
	normalizedPrefix := normalizedCompletionPrefix(prefix)
	segments := resource.SplitPathSegments(normalizedPrefix)
	if len(segments) == 0 {
		return len(spec.Paths) > 0
	}
	for _, item := range spec.Paths {
		if item == nil || item.Template == "" {
			continue
		}
		template := resource.NormalizePath(item.Template)
		if containsWildcardSegment(template) {
			continue
		}
		templateSegments := resource.SplitPathSegments(template)
		if len(templateSegments) < len(segments) {
			continue
		}
		if segmentsMatchBase(templateSegments, segments) {
			return true
		}
	}
	return false
}

func specCollectionPath(spec *openapi.Spec, prefix string) bool {
	if spec == nil {
		return false
	}
	normalizedPrefix := normalizedCompletionPrefix(prefix)
	segments := resource.SplitPathSegments(normalizedPrefix)
	if len(segments) == 0 {
		return false
	}
	for _, item := range spec.Paths {
		if item == nil || item.Template == "" {
			continue
		}
		template := resource.NormalizePath(item.Template)
		if containsWildcardSegment(template) {
			continue
		}
		templateSegments := resource.SplitPathSegments(template)
		if len(templateSegments) <= len(segments) {
			continue
		}
		if !segmentsMatchBase(templateSegments, segments) {
			continue
		}
		nextSegment := templateSegments[len(segments)]
		if isOpenAPIPathParameter(nextSegment) {
			return true
		}
	}
	return false
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
