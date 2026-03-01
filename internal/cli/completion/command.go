package completion

import (
	"github.com/crmarques/declarest/internal/cli/shared"
	"github.com/spf13/cobra"
)

func NewCommand(deps shared.CommandDependencies, globalFlags *shared.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	command := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(
		newBashCommand(),
		newZshCommand(),
		newFishCommand(),
		newPowerShellCommand(),
	)

	return command
}
