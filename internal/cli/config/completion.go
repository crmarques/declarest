package config

import (
	"context"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

func registerSingleContextArgCompletion(command *cobra.Command, deps cliutil.CommandDependencies) {
	command.ValidArgsFunction = func(
		_ *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContextNames(deps, toComplete)
	}
}

func registerRenameFromArgCompletion(command *cobra.Command, deps cliutil.CommandDependencies) {
	command.ValidArgsFunction = func(
		_ *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeContextNames(deps, toComplete)
	}
}

func completeContextNames(deps cliutil.CommandDependencies, toComplete string) ([]string, cobra.ShellCompDirective) {
	service, err := cliutil.RequireContexts(deps)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	items, err := service.List(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return cliutil.CompleteValues(names, toComplete)
}
