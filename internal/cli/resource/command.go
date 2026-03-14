package resource

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	metadatacmd "github.com/crmarques/declarest/internal/cli/metadata"
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
	commandmeta.MarkRequiresContextBootstrap(command)

	getCommand := newGetCommand(deps, globalFlags)
	saveCommand := newSaveCommand(deps)
	applyCommand := newApplyCommand(deps, globalFlags)
	createCommand := newCreateCommand(deps, globalFlags)
	updateCommand := newUpdateCommand(deps, globalFlags)
	deleteCommand := newDeleteCommand(deps)
	diffCommand := newDiffCommand(deps, globalFlags)
	listCommand := newListCommand(deps, globalFlags)
	editCommand := newEditCommand(deps, globalFlags)
	copyCommand := newCopyCommand(deps, globalFlags)
	metadataCommand := metadatacmd.NewCommand(deps, globalFlags)
	defaultsCommand := newDefaultsCommand(deps, globalFlags)
	explainCommand := newExplainCommand(deps, globalFlags)
	describeCommand := newDescribeCommand(deps, globalFlags)
	templateCommand := newTemplateCommand(deps, globalFlags)
	requestCommand := newRequestCommand(deps, globalFlags)

	commandmeta.MarkEmitsExecutionStatus(saveCommand)
	commandmeta.MarkEmitsExecutionStatus(applyCommand)
	commandmeta.MarkEmitsExecutionStatus(createCommand)
	commandmeta.MarkEmitsExecutionStatus(updateCommand)
	commandmeta.MarkEmitsExecutionStatus(deleteCommand)
	commandmeta.MarkEmitsExecutionStatus(editCommand)
	commandmeta.MarkEmitsExecutionStatus(copyCommand)
	commandmeta.MarkTextDefaultStructuredOutput(diffCommand)

	command.AddCommand(
		getCommand,
		saveCommand,
		applyCommand,
		createCommand,
		updateCommand,
		deleteCommand,
		diffCommand,
		listCommand,
		editCommand,
		copyCommand,
		metadataCommand,
		defaultsCommand,
		explainCommand,
		describeCommand,
		templateCommand,
		requestCommand,
	)

	return command
}
