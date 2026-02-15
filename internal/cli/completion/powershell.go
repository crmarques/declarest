package completion

import "github.com/spf13/cobra"

func newPowerShellCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Root().GenPowerShellCompletionWithDesc(command.OutOrStdout())
		},
	}
}
