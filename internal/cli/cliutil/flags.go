package cliutil

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	Context         string
	Verbose         int
	VerboseInsecure bool
	NoStatus        bool
	NoColor         bool
	Output          string
}

type InputFlags struct {
	Payload     string
	ContentType string
}

func BindGlobalFlags(command *cobra.Command, flags *GlobalFlags) {
	command.PersistentFlags().StringVarP(&flags.Context, "context", "c", "", "context name")

	vf := command.PersistentFlags().VarPF(
		newVerboseFlag(&flags.Verbose),
		"verbose", "v",
		"verbosity level: -v info, -vv detail, -vvv trace (or --verbose=N where N is 1-3)",
	)
	vf.NoOptDefVal = "+1"

	command.PersistentFlags().BoolVar(
		&flags.VerboseInsecure,
		"verbose-insecure", false,
		"show secrets, tokens, and credentials in verbose output (requires -v)",
	)

	command.PersistentFlags().BoolVarP(&flags.NoStatus, "no-status", "n", false, "hide status output")
	command.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "disable color output")
	command.PersistentFlags().StringVarP(&flags.Output, "output", "o", OutputAuto, "output format: auto|text|json|yaml")
	RegisterOutputFlagCompletion(command)
}

// IsVerbose returns true when verbosity level is >= 1.
func IsVerbose(flags *GlobalFlags) bool {
	return flags != nil && flags.Verbose >= 1
}

// VerboseLevel returns the resolved verbosity level (0-3).
func VerboseLevel(flags *GlobalFlags) int {
	if flags == nil {
		return 0
	}
	level := flags.Verbose
	if level > 3 {
		level = 3
	}
	return level
}

func BindInputFlags(command *cobra.Command, flags *InputFlags) {
	command.Flags().StringVarP(&flags.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(&flags.ContentType, "content-type", "", "input content type: json|yaml")
	RegisterInputContentTypeFlagCompletion(command)
}

func BindResourceInputFlags(command *cobra.Command, flags *InputFlags) {
	command.Flags().StringVarP(&flags.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(
		&flags.ContentType,
		"content-type",
		"",
		"input content type: json|yaml|xml|hcl|ini|properties|text|binary",
	)
	RegisterResourceInputContentTypeFlagCompletion(command)
}

func BindPathFlag(command *cobra.Command, path *string) {
	command.Flags().StringVarP(path, "path", "p", "", "resource path")
}

// verboseFlag is a custom pflag.Value that supports both -v/-vv/-vvv stacking
// and explicit --verbose=N syntax.
type verboseFlag int

func newVerboseFlag(p *int) *verboseFlag {
	return (*verboseFlag)(p)
}

func (f *verboseFlag) String() string {
	return strconv.Itoa(int(*f))
}

func (f *verboseFlag) Set(s string) error {
	if s == "+1" {
		*f++
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid verbosity level %q: must be 1, 2, or 3", s)
	}
	if v < 0 || v > 3 {
		return fmt.Errorf("verbosity level must be between 0 and 3, got %d", v)
	}
	*f = verboseFlag(v)
	return nil
}

// Type returns "count" so pflag allows -v/-vv/-vvv short flag stacking.
func (f *verboseFlag) Type() string {
	return "count"
}
