package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type contextLister interface {
	ListContexts() ([]string, error)
}

func registerContextNameArgumentCompletion(cmd *cobra.Command, manager contextLister, allowFileCompletions bool, targetArgIndex int) {
	if manager == nil || cmd == nil {
		return
	}
	cmd.ValidArgsFunction = contextNameArgumentCompletion(manager, allowFileCompletions, targetArgIndex)
}

func contextNameArgumentCompletion(manager contextLister, allowFileCompletions bool, targetArgIndex int) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != targetArgIndex {
			return nil, cobra.ShellCompDirectiveDefault
		}
		matches, err := contextNameMatches(manager, toComplete)
		if err != nil || len(matches) == 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		if allowFileCompletions {
			return matches, cobra.ShellCompDirectiveDefault
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

func registerContextNameFlagCompletion(cmd *cobra.Command, manager contextLister, flagName string) {
	if manager == nil || cmd == nil {
		return
	}
	flag := cmd.Flag(flagName)
	if flag == nil {
		return
	}
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		matches, err := contextNameMatches(manager, toComplete)
		if err != nil || len(matches) == 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	})
}

func contextNameMatches(manager contextLister, prefix string) ([]string, error) {
	raw := strings.TrimSpace(prefix)
	names, err := manager.ListContexts()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	matches := make([]string, 0, len(names))
	for _, candidate := range names {
		if raw == "" || strings.HasPrefix(candidate, raw) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Strings(matches)
	return matches, nil
}
