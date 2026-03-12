package completion

import (
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	command := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Args:  cobra.NoArgs,
	}

	bashCommand := newBashCommand()
	zshCommand := newZshCommand()
	fishCommand := newFishCommand()
	powerShellCommand := newPowerShellCommand()
	commandmeta.MarkTextOnlyOutput(bashCommand)
	commandmeta.MarkTextOnlyOutput(zshCommand)
	commandmeta.MarkTextOnlyOutput(fishCommand)
	commandmeta.MarkTextOnlyOutput(powerShellCommand)

	command.AddCommand(
		bashCommand,
		zshCommand,
		fishCommand,
		powerShellCommand,
	)

	return command
}
