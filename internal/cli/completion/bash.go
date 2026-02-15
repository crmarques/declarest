package completion

import "github.com/spf13/cobra"

func newBashCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate Bash completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Root().GenBashCompletion(command.OutOrStdout())
		},
	}
}
