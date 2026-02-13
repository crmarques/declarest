package completion

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring) *cobra.Command {
	_ = deps

	command := common.NewPlaceholderCommand("completion")
	command.AddCommand(
		newBashCommand(),
		newZshCommand(),
		newFishCommand(),
		newPowerShellCommand(),
	)

	return command
}
