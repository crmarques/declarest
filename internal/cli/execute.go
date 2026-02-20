package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
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
}

func (d Dependencies) commandDependencies() common.CommandDependencies {
	return common.CommandDependencies{
		Orchestrator:   d.Orchestrator,
		Contexts:       d.Contexts,
		ResourceStore:  d.ResourceStore,
		RepositorySync: d.RepositorySync,
		Metadata:       d.Metadata,
		Secrets:        d.Secrets,
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
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
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
	switch strings.TrimSpace(path) {
	case "declarest resource save",
		"declarest resource apply",
		"declarest resource create",
		"declarest resource update",
		"declarest resource delete",
		"declarest ad-hoc get",
		"declarest ad-hoc head",
		"declarest ad-hoc options",
		"declarest ad-hoc post",
		"declarest ad-hoc put",
		"declarest ad-hoc patch",
		"declarest ad-hoc delete",
		"declarest ad-hoc trace",
		"declarest ad-hoc connect":
		return true
	default:
		return false
	}
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
