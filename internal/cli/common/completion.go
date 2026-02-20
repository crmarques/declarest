package common

import (
	"context"
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
	pathCompletionDirective  = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
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

type completionDataSource uint8

const (
	completionSourceNone completionDataSource = iota
	completionSourceLocal
	completionSourceRemote
)

type completionSourceStrategy struct {
	primary      completionDataSource
	secondary    completionDataSource
	fallbackOnly bool
}

func defaultCompletionSourceStrategy() completionSourceStrategy {
	return completionSourceStrategy{
		primary:      completionSourceLocal,
		secondary:    completionSourceRemote,
		fallbackOnly: true,
	}
}

func resolveCompletionSourceStrategy(command *cobra.Command) completionSourceStrategy {
	strategy := defaultCompletionSourceStrategy()
	if command == nil {
		return strategy
	}

	parentName := ""
	if command.Parent() != nil {
		parentName = command.Parent().Name()
	}

	switch parentName {
	case "resource":
		switch command.Name() {
		case "get", "save":
			strategy.primary = completionSourceRemote
			strategy.secondary = completionSourceLocal
			strategy.fallbackOnly = true

			if commandFlagEnabled(command, "repository") {
				strategy.primary = completionSourceLocal
				strategy.secondary = completionSourceRemote
			}
			if commandFlagEnabled(command, "remote-server") {
				strategy.primary = completionSourceRemote
				strategy.secondary = completionSourceLocal
			}
		case "list":
			strategy.primary = completionSourceRemote
			strategy.secondary = completionSourceLocal
			strategy.fallbackOnly = true

			if commandFlagEnabled(command, "repository") {
				strategy.primary = completionSourceLocal
				strategy.secondary = completionSourceRemote
			}
			if commandFlagEnabled(command, "remote-server") {
				strategy.primary = completionSourceRemote
				strategy.secondary = completionSourceLocal
			}
		case "delete":
			strategy.fallbackOnly = true
			switch {
			case commandFlagEnabled(command, "both"):
				strategy.primary = completionSourceLocal
				strategy.secondary = completionSourceRemote
				strategy.fallbackOnly = false
			case commandFlagEnabled(command, "repository"):
				strategy.primary = completionSourceLocal
				strategy.secondary = completionSourceRemote
			default:
				strategy.primary = completionSourceRemote
				strategy.secondary = completionSourceLocal
			}
		case "apply", "create", "update", "diff", "explain", "template":
			strategy.primary = completionSourceLocal
			strategy.secondary = completionSourceRemote
			strategy.fallbackOnly = true
		}
	case "ad-hoc":
		strategy.primary = completionSourceRemote
		strategy.secondary = completionSourceLocal
		strategy.fallbackOnly = true
	case "metadata", "secret":
		strategy.primary = completionSourceLocal
		strategy.secondary = completionSourceRemote
		strategy.fallbackOnly = true
	}

	return strategy
}

func commandFlagEnabled(command *cobra.Command, flagName string) bool {
	if command == nil || command.Flags() == nil {
		return false
	}
	flag := command.Flags().Lookup(flagName)
	if flag == nil {
		return false
	}
	enabled, err := command.Flags().GetBool(flagName)
	if err != nil {
		return false
	}
	return enabled
}

func shouldQuerySecondarySource(
	strategy completionSourceStrategy,
	primaryCount int,
	primaryErr error,
) bool {
	if strategy.secondary == completionSourceNone || strategy.secondary == strategy.primary {
		return false
	}
	if !strategy.fallbackOnly {
		return true
	}
	if primaryErr != nil {
		return true
	}
	return primaryCount == 0
}

func listCompletionResources(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	source completionDataSource,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	switch source {
	case completionSourceLocal:
		return orchestratorService.ListLocal(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	case completionSourceRemote:
		return orchestratorService.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	default:
		return nil, nil
	}
}

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
		return filterPathSuggestions(suggestions, toComplete), pathCompletionDirective
	}

	ctx, cancel := completionContext(command.Context())
	defer cancel()

	strategy := resolveCompletionSourceStrategy(command)
	normalizedPrefix := normalizeCompletionPrefix(toComplete)
	queryPath := completionQueryPath(normalizedPrefix)
	localItems := []resource.Resource{}
	remoteItems := []resource.Resource{}
	primarySourceSuggestions := map[string]struct{}{}

	primaryItems, _, primaryErr := queryScopedCompletionResources(
		ctx,
		orchestratorService,
		strategy.primary,
		queryPath,
		suggestions,
	)
	switch strategy.primary {
	case completionSourceLocal:
		localItems = append(localItems, primaryItems...)
	case completionSourceRemote:
		remoteItems = append(remoteItems, primaryItems...)
	}
	addResourceSuggestions(primarySourceSuggestions, primaryItems)

	openAPISpec, err := orchestratorService.GetOpenAPISpec(ctx)
	if err == nil {
		addOpenAPISuggestions(suggestions, openAPISpec, normalizedPrefix)
		if shouldRunSmartOpenAPISuggestions(suggestions, toComplete) {
			addSmartOpenAPISuggestions(
				ctx,
				orchestratorService,
				suggestions,
				localItems,
				remoteItems,
				openAPISpec,
				toComplete,
				strategy,
			)
		}
	}

	if shouldRunRootRecursiveFallback(suggestions, toComplete) {
		primaryRootItems, primaryRootErr := listCompletionResources(
			ctx,
			orchestratorService,
			strategy.primary,
			"/",
			true,
		)
		if primaryRootErr == nil {
			addResourceSuggestions(suggestions, primaryRootItems)
			addResourceSuggestions(primarySourceSuggestions, primaryRootItems)
			switch strategy.primary {
			case completionSourceLocal:
				localItems = append(localItems, primaryRootItems...)
			case completionSourceRemote:
				remoteItems = append(remoteItems, primaryRootItems...)
			}
		}
	}

	if queryPath == "/" && primaryErr == nil && len(filterPathSuggestions(primarySourceSuggestions, toComplete)) == 0 {
		primaryRootItems, primaryRootErr := listCompletionResources(
			ctx,
			orchestratorService,
			strategy.primary,
			"/",
			true,
		)
		if primaryRootErr == nil {
			addResourceSuggestions(suggestions, primaryRootItems)
			addResourceSuggestions(primarySourceSuggestions, primaryRootItems)
			switch strategy.primary {
			case completionSourceLocal:
				localItems = append(localItems, primaryRootItems...)
			case completionSourceRemote:
				remoteItems = append(remoteItems, primaryRootItems...)
			}
		}
	}

	primaryCompletionCount := len(filterPathSuggestions(primarySourceSuggestions, toComplete))
	if shouldQuerySecondarySource(strategy, primaryCompletionCount, primaryErr) {
		secondaryItems, _, secondaryErr := queryScopedCompletionResources(
			ctx,
			orchestratorService,
			strategy.secondary,
			queryPath,
			suggestions,
		)
		if secondaryErr == nil {
			switch strategy.secondary {
			case completionSourceLocal:
				localItems = append(localItems, secondaryItems...)
			case completionSourceRemote:
				remoteItems = append(remoteItems, secondaryItems...)
			}
		}

		if shouldRunRootRecursiveFallback(suggestions, toComplete) {
			secondaryRootItems, secondaryRootErr := listCompletionResources(
				ctx,
				orchestratorService,
				strategy.secondary,
				"/",
				true,
			)
			if secondaryRootErr == nil {
				addResourceSuggestions(suggestions, secondaryRootItems)
			}
		}
	}

	return filterPathSuggestions(suggestions, toComplete), pathCompletionDirective
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
