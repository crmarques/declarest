package resource

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
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

func normalizeReadSourceSelection(sourceFlag string, fromRepository bool, fromRemoteServer bool) (string, error) {
	normalized, err := normalizeSourceSelection(sourceFlag, false, fromRepository, fromRemoteServer, false)
	if err != nil {
		return "", err
	}
	if normalized == sourceBoth {
		return "", common.ValidationError("flag --source must be one of: remote-server, repository", nil)
	}
	return normalized, nil
}

func normalizeDeleteSourceSelection(sourceFlag string, fromRepository bool, fromRemoteServer bool, fromBoth bool) (string, error) {
	return normalizeSourceSelection(sourceFlag, true, fromRepository, fromRemoteServer, fromBoth)
}

func normalizeSourceSelection(
	sourceFlag string,
	allowBoth bool,
	fromRepository bool,
	fromRemoteServer bool,
	fromBoth bool,
) (string, error) {
	sourceValue := strings.TrimSpace(sourceFlag)
	if sourceValue != "" {
		if fromRepository || fromRemoteServer || fromBoth {
			return "", common.ValidationError(
				"flag --source cannot be combined with legacy source flags (--repository, --remote-server, --both)",
				nil,
			)
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
			return "", common.ValidationError("flag --source must be one of: remote-server, repository, both", nil)
		}
		return "", common.ValidationError("flag --source must be one of: remote-server, repository", nil)
	}

	if fromRepository && fromRemoteServer {
		return "", common.ValidationError("flags --repository and --remote-server cannot be used together", nil)
	}
	if allowBoth {
		explicitSources := 0
		if fromRepository {
			explicitSources++
		}
		if fromRemoteServer {
			explicitSources++
		}
		if fromBoth {
			explicitSources++
		}
		if explicitSources > 1 {
			return "", common.ValidationError("flags --repository, --remote-server, and --both are mutually exclusive", nil)
		}
		if fromBoth {
			return sourceBoth, nil
		}
	}

	if fromRepository {
		return sourceRepository, nil
	}
	return sourceRemoteServer, nil
}

func bindReadSourceFlags(command *cobra.Command, sourceFlag *string, fromRepository *bool, fromRemoteServer *bool) {
	command.Flags().StringVar(sourceFlag, "source", "", "read/list source: remote-server or repository (default: remote-server)")
	common.RegisterFlagValueCompletions(command, "source", readSourceCompletionValues)

	command.Flags().BoolVar(fromRepository, "repository", false, "read/list from repository (legacy alias for --source repository)")
	command.Flags().BoolVar(fromRemoteServer, "remote-server", false, "read/list from remote server (legacy alias for --source remote-server)")
	_ = command.Flags().MarkHidden("repository")
	_ = command.Flags().MarkHidden("remote-server")
}

func bindDeleteSourceFlags(command *cobra.Command, sourceFlag *string, fromRepository *bool, fromRemoteServer *bool, fromBoth *bool) {
	command.Flags().StringVar(sourceFlag, "source", "", "delete source: remote-server, repository, or both (default: remote-server)")
	common.RegisterFlagValueCompletions(command, "source", deleteSourceCompletionValues)

	command.Flags().BoolVar(fromRepository, "repository", false, "delete from repository (legacy alias for --source repository)")
	command.Flags().BoolVar(fromRemoteServer, "remote-server", false, "delete from remote server (legacy alias for --source remote-server)")
	command.Flags().BoolVar(fromBoth, "both", false, "delete from both remote server and repository (legacy alias for --source both)")
	_ = command.Flags().MarkHidden("repository")
	_ = command.Flags().MarkHidden("remote-server")
	_ = command.Flags().MarkHidden("both")
}

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
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
