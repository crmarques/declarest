package resource

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

const (
	sourceRepository    = "repository"
	sourceManagedServer = "managed-server"
	sourceBoth          = "both"
)

var (
	readSourceCompletionValues   = []string{sourceManagedServer, sourceRepository}
	deleteSourceCompletionValues = []string{sourceManagedServer, sourceRepository, sourceBoth}
)

func normalizeReadSourceSelection(sourceFlag string) (string, error) {
	return normalizeSourceSelection(sourceFlag, false)
}

func normalizeDeleteSourceSelection(sourceFlag string) (string, error) {
	return normalizeSourceSelection(sourceFlag, true)
}

func normalizeSourceSelection(sourceFlag string, allowBoth bool) (string, error) {
	sourceValue := strings.TrimSpace(sourceFlag)
	if sourceValue == "" {
		return sourceManagedServer, nil
	}

	switch sourceValue {
	case sourceRepository, sourceManagedServer:
		return sourceValue, nil
	case sourceBoth:
		if allowBoth {
			return sourceValue, nil
		}
	}

	if allowBoth {
		return "", cliutil.ValidationError("flag --source must be one of: managed-server, repository, both", nil)
	}
	return "", cliutil.ValidationError("flag --source must be one of: managed-server, repository", nil)
}

func bindReadSourceFlags(command *cobra.Command, sourceFlag *string) {
	command.Flags().StringVar(sourceFlag, "source", "", "read/list source: managed-server or repository (default: managed-server)")
	cliutil.RegisterFlagValueCompletions(command, "source", readSourceCompletionValues)
}

func bindDeleteSourceFlags(command *cobra.Command, sourceFlag *string) {
	command.Flags().StringVar(sourceFlag, "source", "", "delete source: managed-server, repository, or both (default: managed-server)")
	cliutil.RegisterFlagValueCompletions(command, "source", deleteSourceCompletionValues)
}

func bindSkipItemsFlag(command *cobra.Command, skipItemsFlag *string) {
	command.Flags().StringVar(skipItemsFlag, "skip-items", "", "comma-separated collection items to exclude by alias, id, or path segment")
}

func parseSkipItemsFlag(command *cobra.Command, rawValue string) ([]string, error) {
	flag := command.Flags().Lookup("skip-items")
	if flag == nil || !flag.Changed {
		return nil, nil
	}

	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return nil, cliutil.ValidationError("flag --skip-items requires at least one collection item", nil)
	}

	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, rawItem := range parts {
		item := strings.TrimSpace(rawItem)
		if item == "" {
			return nil, cliutil.ValidationError("flag --skip-items contains an empty collection item", nil)
		}
		if _, found := seen[item]; found {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}

	return items, nil
}

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "resource",
		Short: "Manage resources",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(deps, globalFlags),
		newSaveCommand(deps),
		newApplyCommand(deps, globalFlags),
		newCreateCommand(deps, globalFlags),
		newUpdateCommand(deps, globalFlags),
		newDeleteCommand(deps),
		newDiffCommand(deps, globalFlags),
		newListCommand(deps, globalFlags),
		newEditCommand(deps, globalFlags),
		newCopyCommand(deps, globalFlags),
		newExplainCommand(deps, globalFlags),
		newTemplateCommand(deps, globalFlags),
		newRequestCommand(deps, globalFlags),
	)

	return command
}
