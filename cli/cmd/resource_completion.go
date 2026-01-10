package cmd

import (
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
	suggestions := gatherPathSuggestions(recon, trimmed, sources)
	if len(suggestions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func gatherPathSuggestions(recon *reconciler.DefaultReconciler, prefix string, sources []pathCompletionSource) []string {
	set := make(map[string]struct{})
	for _, source := range sources {
		switch source {
		case pathCompletionSourceRepo:
			for _, candidate := range repoPathSuggestions(recon, prefix) {
				set[candidate] = struct{}{}
			}
		case pathCompletionSourceRemote:
			for _, candidate := range remotePathSuggestions(recon, prefix) {
				set[candidate] = struct{}{}
			}
		}
	}

	for _, candidate := range openAPIPathSuggestions(recon.ResourceRecordProvider, prefix) {
		set[candidate] = struct{}{}
	}

	return mapKeysSorted(set)
}

func repoPathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []string {
	return filterPathsByPrefix(recon.RepositoryResourcePaths(), prefix)
}

func remotePathSuggestions(recon *reconciler.DefaultReconciler, prefix string) []string {
	collection := remoteCompletionCollection(prefix)
	paths, err := recon.ListRemoteResourcePaths(collection)
	if err != nil {
		return nil
	}
	return filterPathsByPrefix(paths, prefix)
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

func openAPIPathSuggestions(provider interface{}, prefix string) []string {
	spec := openapiSpecFromProvider(provider)
	if spec == nil {
		return nil
	}
	return specPathSuggestions(spec, prefix)
}

func specPathSuggestions(spec *openapi.Spec, prefix string) []string {
	if spec == nil {
		return nil
	}
	segments := resource.SplitPathSegments(strings.TrimSpace(prefix))
	results := make(map[string]struct{})
	for _, item := range spec.Paths {
		if item == nil || item.Template == "" {
			continue
		}
		template := resource.NormalizePath(item.Template)
		if matchesSpecPrefix(template, segments) {
			results[template] = struct{}{}
		}
		if collection := wildcardCollectionPath(item.Template); collection != "" && matchesSpecPrefix(collection, segments) {
			results[collection] = struct{}{}
		}
	}
	return mapKeysSorted(results)
}

func matchesSpecPrefix(path string, prefixSegments []string) bool {
	if len(prefixSegments) == 0 {
		return true
	}
	templateSegments := resource.SplitPathSegments(path)
	if len(prefixSegments) > len(templateSegments) {
		return false
	}
	for idx, prefix := range prefixSegments {
		templ := templateSegments[idx]
		if isWildcardSegment(templ) || isOpenAPIPathParameter(templ) {
			continue
		}
		if !strings.HasPrefix(templ, prefix) {
			return false
		}
	}
	return true
}

func mapKeysSorted(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	list := make([]string, 0, len(values))
	for value := range values {
		list = append(list, value)
	}
	sort.Strings(list)
	return list
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
	if boolFlagValue(cmd, "from-repo", false) {
		return []pathCompletionSource{pathCompletionSourceRepo}
	}
	return []pathCompletionSource{pathCompletionSourceRemote}
}

func resourceDeletePathStrategy(cmd *cobra.Command) []pathCompletionSource {
	sources := make([]pathCompletionSource, 0, 2)
	if boolFlagValue(cmd, "repo", true) {
		sources = append(sources, pathCompletionSourceRepo)
	}
	if boolFlagValue(cmd, "remote", false) {
		sources = append(sources, pathCompletionSourceRemote)
	}
	if len(sources) == 0 {
		return []pathCompletionSource{pathCompletionSourceRepo}
	}
	return sources
}

func resourceListPathStrategy(cmd *cobra.Command) []pathCompletionSource {
	sources := make([]pathCompletionSource, 0, 2)
	if boolFlagValue(cmd, "repo", true) {
		sources = append(sources, pathCompletionSourceRepo)
	}
	if boolFlagValue(cmd, "remote", false) {
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
