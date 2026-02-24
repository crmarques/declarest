package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Dependencies struct {
	Orchestrator   orchestrator.Orchestrator
	Contexts       config.ContextService
	ResourceStore  repository.ResourceStore
	RepositorySync repository.RepositorySync
	Metadata       metadata.MetadataService
	Secrets        secrets.SecretProvider
	ResourceServer server.ResourceServer
}

func (d Dependencies) commandDependencies() common.CommandDependencies {
	return common.CommandDependencies{
		Orchestrator:   d.Orchestrator,
		Contexts:       d.Contexts,
		ResourceStore:  d.ResourceStore,
		RepositorySync: d.RepositorySync,
		Metadata:       d.Metadata,
		Secrets:        d.Secrets,
		ResourceServer: d.ResourceServer,
	}
}

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
	if err == nil {
		return 0
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return 1
	}

	switch typedErr.Category {
	case faults.ValidationError:
		return 2
	case faults.NotFoundError:
		return 3
	case faults.AuthError:
		return 4
	case faults.ConflictError:
		return 5
	case faults.TransportError:
		return 6
	default:
		return 1
	}
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
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return true
	}
	return hasNoColorArgToken(args)
}

func shouldEmitExecutionStatus(args []string, command *cobra.Command) bool {
	if shouldSuppressStatusMessage(args) {
		return false
	}
	if isHelpOrCompletionInvocation(args) {
		return false
	}
	return commandPathSupportsExecutionStatus(commandPath(command))
}

func commandPath(command *cobra.Command) string {
	if command == nil {
		return ""
	}
	return strings.TrimSpace(command.CommandPath())
}

func commandPathSupportsExecutionStatus(path string) bool {
	return commandmeta.EmitsExecutionStatusPath(path)
}

func shouldSuppressStatusMessage(args []string) bool {
	flags := pflag.NewFlagSet("status", pflag.ContinueOnError)
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.SetOutput(io.Discard)

	var noStatus bool
	flags.BoolVarP(&noStatus, "no-status", "n", false, "hide status output")
	if err := flags.Parse(args); err != nil {
		return hasNoStatusArgToken(args)
	}
	return noStatus
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

func hasNoStatusArgToken(args []string) bool {
	for _, current := range args {
		if current == "--no-status" || current == "-n" {
			return true
		}
		if strings.HasPrefix(current, "--no-status=") {
			return strings.TrimSpace(strings.TrimPrefix(current, "--no-status=")) != "false"
		}
	}
	return false
}

func hasNoColorArgToken(args []string) bool {
	for _, current := range args {
		if current == "--no-color" {
			return true
		}
		if strings.HasPrefix(current, "--no-color=") {
			return strings.TrimSpace(strings.TrimPrefix(current, "--no-color=")) != "false"
		}
	}
	return false
}
