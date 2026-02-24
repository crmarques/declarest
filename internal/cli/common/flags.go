package common

import "github.com/spf13/cobra"

type GlobalFlags struct {
	Context  string
	Debug    bool
	Verbose  bool
	NoStatus bool
	NoColor  bool
	Output   string
}

type InputFlags struct {
	Payload string
	Format  string
}

func BindGlobalFlags(command *cobra.Command, flags *GlobalFlags) {
	command.PersistentFlags().StringVarP(&flags.Context, "context", "c", "", "context name")
	command.PersistentFlags().BoolVarP(&flags.Debug, "debug", "d", false, "enable debug output")
	command.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "show complementary command output")
	command.PersistentFlags().BoolVarP(&flags.NoStatus, "no-status", "n", false, "hide status output")
	command.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "disable color output")
	command.PersistentFlags().StringVarP(&flags.Output, "output", "o", OutputAuto, "output format: auto|text|json|yaml")
	RegisterOutputFlagCompletion(command)
}

func IsVerbose(flags *GlobalFlags) bool {
	return flags != nil && flags.Verbose
}

func BindInputFlags(command *cobra.Command, flags *InputFlags) {
	command.Flags().StringVarP(&flags.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVarP(&flags.Format, "format", "i", OutputJSON, "input format: json|yaml")
	RegisterInputFormatFlagCompletion(command)
}

func BindPathFlag(command *cobra.Command, path *string) {
	command.Flags().StringVarP(path, "path", "p", "", "resource path")
}
