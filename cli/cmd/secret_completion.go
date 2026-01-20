package cmd

import (
	"sort"
	"strings"

	"github.com/crmarques/declarest/resource"

	"github.com/spf13/cobra"
)

func registerSecretPathAndKeyCompletion(cmd *cobra.Command) {
	if cmd == nil {
		return
	}

	validator := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeResourcePathCandidates(cmd, toComplete, resourceRepoPathStrategy)
		case 1:
			path := secretCompletionPath(cmd, args)
			if path == "" {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return secretKeyCompletion(path, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveDefault
		}
	}

	cmd.ValidArgsFunction = validator

	if cmd.Flag("path") != nil {
		_ = cmd.RegisterFlagCompletionFunc("path", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeResourcePathCandidates(cmd, toComplete, resourceRepoPathStrategy)
		})
	}

	if cmd.Flag("key") != nil {
		_ = cmd.RegisterFlagCompletionFunc("key", secretKeyFlagCompletion)
	}
}

func secretCompletionPath(cmd *cobra.Command, args []string) string {
	if len(args) > 0 {
		if trimmed := strings.TrimSpace(args[0]); trimmed != "" {
			return trimmed
		}
	}
	if flag := cmd.Flags().Lookup("path"); flag != nil {
		if trimmed := strings.TrimSpace(flag.Value.String()); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func secretKeyCompletion(path, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.TrimSpace(path) == "" {
		return nil, cobra.ShellCompDirectiveDefault
	}
	normalized := resource.NormalizePath(path)

	recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}

	keys, err := recon.ListSecretKeys(normalized)
	if err != nil || len(keys) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	sort.Strings(keys)
	prefix := strings.TrimSpace(toComplete)
	matches := make([]string, 0, len(keys))
	for _, key := range keys {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			matches = append(matches, key)
		}
	}
	if len(matches) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

func secretKeyFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	path := secretCompletionPath(cmd, args)
	if path == "" {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return secretKeyCompletion(path, toComplete)
}
