package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Dependencies = cliutil.CommandDependencies

var NewDependencies = cliutil.NewCommandDependencies

func Execute(deps Dependencies) error {
	root := NewRootCommand(deps)
	command, err := root.ExecuteC()
	emitStatus := shouldEmitExecutionStatus(os.Args[1:], command)

	if err != nil {
		if emitStatus {
			writeExecutionErrorStatus(root.ErrOrStderr(), err)
		} else {
			_, _ = fmt.Fprintln(root.ErrOrStderr(), strings.TrimSpace(err.Error()))
		}
		return err
	}
	if emitStatus {
		writeExecutionOKStatus(root.ErrOrStderr())
	}
	return nil
}

func ExitCodeForError(err error) int {
	return faults.ExitCodeForError(err)
}

func writeExecutionOKStatus(w io.Writer) {
	_, _ = fmt.Fprintf(w, "%s command executed successfully.\n", formatStatusLabel(w, "OK"))
}

func writeExecutionErrorStatus(w io.Writer, err error) {
	description := "command execution failed"
	if err != nil {
		description = fmt.Sprintf("%s: %s", description, strings.TrimSpace(err.Error()))
	}
	_, _ = fmt.Fprintf(w, "%s %s.\n", formatStatusLabel(w, "ERROR"), description)
}

func formatStatusLabel(w io.Writer, status string) string {
	label := fmt.Sprintf("[%s]", strings.TrimSpace(status))
	if !supportsANSIStatus(w) {
		return label
	}

	switch strings.TrimSpace(status) {
	case "OK":
		return "\x1b[1;32m" + label + "\x1b[0m"
	case "ERROR":
		return "\x1b[1;31m" + label + "\x1b[0m"
	default:
		return label
	}
}

func supportsANSIStatus(w io.Writer) bool {
	if shouldSuppressColor(os.Args[1:]) {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil || info == nil {
		return false
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	term := strings.TrimSpace(strings.ToLower(os.Getenv("TERM")))
	return term != "" && term != "dumb"
}

func shouldSuppressColor(args []string) bool {
	return parseGlobalBoolFlag(
		args,
		cliutil.GlobalFlagNoColor,
		"",
		cliutil.EnvBoolOrDefault(
			cliutil.GlobalEnvNoColor,
			cliutil.EnvPresentOrDefault(cliutil.GlobalEnvNoColorLegacy, false),
		),
	)
}

func shouldEmitExecutionStatus(args []string, command *cobra.Command) bool {
	if shouldSuppressStatusMessage(args) {
		return false
	}
	if isHelpOrCompletionInvocation(args) {
		return false
	}
	return commandmeta.EmitsExecutionStatus(command)
}

func shouldSuppressStatusMessage(args []string) bool {
	return parseGlobalBoolFlag(
		args,
		cliutil.GlobalFlagNoStatus,
		cliutil.GlobalFlagNoStatusShort,
		cliutil.EnvBoolOrDefault(cliutil.GlobalEnvNoStatus, false),
	)
}

func isHelpOrCompletionInvocation(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if args[0] == "help" {
		return true
	}
	switch args[0] {
	case "completion", "__complete", "__completeNoDesc":
		return true
	}

	for _, current := range args {
		if current == "--" {
			break
		}
		if current == "--help" || current == "-h" {
			return true
		}
	}
	return false
}

func parseGlobalBoolFlag(args []string, name string, shorthand string, defaultValue bool) bool {
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.ParseErrorsAllowlist = pflag.ParseErrorsAllowlist{
		UnknownFlags: true,
	}
	flags.SetOutput(io.Discard)

	var value bool
	if strings.TrimSpace(shorthand) == "" {
		flags.BoolVar(&value, name, defaultValue, "")
	} else {
		flags.BoolVarP(&value, name, shorthand, defaultValue, "")
	}
	if err := flags.Parse(args); err != nil {
		return defaultValue
	}
	return value
}
