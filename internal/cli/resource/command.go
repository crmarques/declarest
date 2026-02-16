package resource

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

const (
	sourceLocal  = "local"
	sourceRemote = "remote"
)

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
		newExplainCommand(deps, globalFlags),
		newTemplateCommand(deps, globalFlags),
	)

	return command
}
