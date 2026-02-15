package completion

import "github.com/spf13/cobra"

func newZshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate Zsh completion",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Root().GenZshCompletion(command.OutOrStdout())
		},
	}
}
