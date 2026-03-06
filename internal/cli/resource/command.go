package resource

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
)

const (
	sourceRepository   = "repository"
	sourceRemoteServer = "remote-server"
	sourceBoth         = "both"
)

var (
	readSourceCompletionValues   = []string{sourceRemoteServer, sourceRepository}
	deleteSourceCompletionValues = []string{sourceRemoteServer, sourceRepository, sourceBoth}
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
		return sourceRemoteServer, nil
	}

	switch sourceValue {
	case sourceRepository, sourceRemoteServer:
		return sourceValue, nil
	case sourceBoth:
		if allowBoth {
			return sourceValue, nil
		}
	}

	if allowBoth {
		return "", cliutil.ValidationError("flag --source must be one of: remote-server, repository, both", nil)
	}
	return "", cliutil.ValidationError("flag --source must be one of: remote-server, repository", nil)
}

func bindReadSourceFlags(command *cobra.Command, sourceFlag *string) {
	command.Flags().StringVar(sourceFlag, "source", "", "read/list source: remote-server or repository (default: remote-server)")
	cliutil.RegisterFlagValueCompletions(command, "source", readSourceCompletionValues)
}

func bindDeleteSourceFlags(command *cobra.Command, sourceFlag *string) {
	command.Flags().StringVar(sourceFlag, "source", "", "delete source: remote-server, repository, or both (default: remote-server)")
	cliutil.RegisterFlagValueCompletions(command, "source", deleteSourceCompletionValues)
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
