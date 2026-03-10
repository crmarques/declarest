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

func bindExcludeFlag(command *cobra.Command, excludeItems *[]string) {
	command.Flags().StringSliceVar(
		excludeItems,
		"exclude",
		nil,
		"repeatable or comma-separated collection items to exclude by alias, id, or path segment",
	)
}

func parseExcludeFlag(command *cobra.Command, rawValues []string) ([]string, error) {
	flag := command.Flags().Lookup("exclude")
	if flag == nil || !flag.Changed {
		return nil, nil
	}

	items := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, rawValue := range rawValues {
		trimmed := strings.TrimSpace(rawValue)
		if trimmed == "" {
			return nil, cliutil.ValidationError("flag --exclude requires at least one collection item", nil)
		}

		for _, rawItem := range strings.Split(trimmed, ",") {
			item := strings.TrimSpace(rawItem)
			if item == "" {
				return nil, cliutil.ValidationError("flag --exclude contains an empty collection item", nil)
			}
			if _, found := seen[item]; found {
				continue
			}
			seen[item] = struct{}{}
			items = append(items, item)
		}
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
		newDescribeCommand(deps, globalFlags),
		newTemplateCommand(deps, globalFlags),
		newRequestCommand(deps, globalFlags),
	)

	return command
}
