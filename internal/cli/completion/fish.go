package completion

import "github.com/spf13/cobra"

func newFishCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate Fish completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Root().GenFishCompletion(command.OutOrStdout(), true)
		},
	}
}
