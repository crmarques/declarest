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

	localItems, err := orchestratorService.ListLocal(ctx, "/", orchestratordomain.ListPolicy{Recursive: true})
	if err == nil {
		addResourceSuggestions(suggestions, localItems)
	}

	remoteItems, err := orchestratorService.ListRemote(ctx, "/", orchestratordomain.ListPolicy{Recursive: true})
	if err == nil {
		addResourceSuggestions(suggestions, remoteItems)
	}

	openAPISpec, err := orchestratorService.GetOpenAPISpec(ctx)
	if err == nil {
		addOpenAPISuggestions(suggestions, openAPISpec)
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
	root, ok := asStringMap(openAPISpec)
	if !ok {
		return
	}

	pathsValue, ok := root["paths"]
	if !ok {
		return
	}

	pathsMap, ok := asStringMap(pathsValue)
	if !ok {
		return
	}

	for pathKey := range pathsMap {
		addPathSuggestion(suggestions, pathKey)
	}
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
	normalizedPrefix := strings.TrimSpace(toComplete)
	if normalizedPrefix != "" && !strings.HasPrefix(normalizedPrefix, "/") {
		normalizedPrefix = "/" + strings.Trim(normalizedPrefix, "/")
	}

	items := make([]string, 0, len(suggestions))
	for value := range suggestions {
		if normalizedPrefix != "" && !strings.HasPrefix(value, normalizedPrefix) {
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
