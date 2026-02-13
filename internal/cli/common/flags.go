package common

import "github.com/spf13/cobra"

type GlobalFlags struct {
	Context  string
	Debug    bool
	NoStatus bool
	Output   string
}

func BindGlobalFlags(command *cobra.Command, flags *GlobalFlags) {
	command.PersistentFlags().StringVar(&flags.Context, "context", "", PlaceholderMessage)
	command.PersistentFlags().BoolVar(&flags.Debug, "debug", false, PlaceholderMessage)
	command.PersistentFlags().BoolVar(&flags.NoStatus, "no-status", false, PlaceholderMessage)
	command.PersistentFlags().StringVar(&flags.Output, "output", "text", PlaceholderMessage)
}
