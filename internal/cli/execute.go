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
	cliutil.WriteStatusLine(w, "OK", "command executed successfully.")
}

func writeExecutionErrorStatus(w io.Writer, err error) {
	description := "command execution failed"
	if err != nil {
		description = fmt.Sprintf("%s: %s", description, strings.TrimSpace(err.Error()))
	}
	cliutil.WriteStatusLine(w, "ERROR", description+".")
}

func shouldSuppressColor(args []string) bool {
	return cliutil.ShouldSuppressColor(args)
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
