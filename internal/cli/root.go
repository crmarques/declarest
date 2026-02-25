package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/internal/cli/completion"
	"github.com/crmarques/declarest/internal/cli/config"
	metadatacmd "github.com/crmarques/declarest/internal/cli/metadata"
	"github.com/crmarques/declarest/internal/cli/repo"
	resourcecmd "github.com/crmarques/declarest/internal/cli/resource"
	resourceservercmd "github.com/crmarques/declarest/internal/cli/resourceserver"
	"github.com/crmarques/declarest/internal/cli/secret"
	"github.com/crmarques/declarest/internal/cli/version"
	"github.com/spf13/cobra"
)

const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .LocalNonPersistentFlags.HasAvailableFlags}}

Flags:
{{.LocalNonPersistentFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if or .HasAvailableInheritedFlags .HasAvailablePersistentFlags}}

Global Flags:
{{if .HasAvailableInheritedFlags}}{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}{{if and .HasAvailableInheritedFlags .HasAvailablePersistentFlags}}
{{end}}{{if .HasAvailablePersistentFlags}}{{.PersistentFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}
{{if .HasAvailableSubCommands}}Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func NewRootCommand(deps Dependencies) *cobra.Command {
	commandDeps := deps.commandDependencies()
	var globalFlags common.GlobalFlags

	root := &cobra.Command{
		Use:   "declarest",
		Short: "Manage declarative resources",
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
		Args: cobra.NoArgs,
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			if err := common.ValidateOutputFormat(globalFlags.Output); err != nil {
				return err
			}
			if err := common.ValidateOutputFormatForCommandPath(command.CommandPath(), globalFlags.Output); err != nil {
				return err
			}

			commandContext := context.Background()
			commandContext = common.WithContextName(commandContext, globalFlags.Context)
			commandContext = debugctx.WithEnabled(commandContext, globalFlags.Debug)
			commandContext = debugctx.WithWriter(commandContext, command.ErrOrStderr())
			command.SetContext(commandContext)

			debugctx.Printf(
				command.Context(),
				"root flags context=%q output=%q verbose=%t no_status=%t no_color=%t command=%q",
				globalFlags.Context,
				globalFlags.Output,
				globalFlags.Verbose,
				globalFlags.NoStatus,
				globalFlags.NoColor,
				command.CommandPath(),
			)

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetUsageTemplate(usageTemplate)
	defaultHelpFunc := root.HelpFunc()
	root.SetHelpFunc(func(command *cobra.Command, args []string) {
		originalOut := command.OutOrStdout()
		originalErr := command.ErrOrStderr()

		buffer := &bytes.Buffer{}
		command.SetOut(buffer)
		command.SetErr(buffer)
		defaultHelpFunc(command, args)
		command.SetOut(originalOut)
		command.SetErr(originalErr)

		rendered := strings.TrimRight(buffer.String(), "\n")
		if rendered == "" {
			_, _ = fmt.Fprintln(originalOut)
			return
		}

		_, _ = fmt.Fprintln(originalOut, rendered)
	})

	common.BindGlobalFlags(root, &globalFlags)
	common.RegisterContextFlagCompletion(root, commandDeps)
	root.PersistentFlags().BoolP("help", "h", false, "help for command")

	root.AddGroup(
		&cobra.Group{ID: "basic", Title: "Basic Commands:"},
		&cobra.Group{ID: "other", Title: "Other Commands:"},
	)

	basicCommands := []*cobra.Command{
		config.NewCommand(commandDeps, &globalFlags),
		metadatacmd.NewCommand(commandDeps, &globalFlags),
		repo.NewCommand(commandDeps, &globalFlags),
		resourcecmd.NewCommand(commandDeps, &globalFlags),
		resourceservercmd.NewCommand(commandDeps),
		secret.NewCommand(commandDeps, &globalFlags),
	}
	for _, command := range basicCommands {
		command.GroupID = "basic"
		root.AddCommand(command)
	}

	otherCommands := []*cobra.Command{
		completion.NewCommand(commandDeps, &globalFlags),
		version.NewCommand(commandDeps, &globalFlags),
	}
	for _, command := range otherCommands {
		command.GroupID = "other"
		root.AddCommand(command)
	}

	wrapUsageForMissingPositionalParameterErrors(root)

	return root
}

func wrapUsageForMissingPositionalParameterErrors(root *cobra.Command) {
	if root == nil {
		return
	}

	var wrapCommandTree func(*cobra.Command)
	wrapCommandTree = func(command *cobra.Command) {
		if command == nil {
			return
		}

		command.Args = wrapCommandErrorHandlerWithUsage(command.Args)
		command.PersistentPreRunE = wrapCommandErrorHandlerWithUsage(command.PersistentPreRunE)
		command.PreRunE = wrapCommandErrorHandlerWithUsage(command.PreRunE)
		command.RunE = wrapCommandErrorHandlerWithUsage(command.RunE)

		for _, child := range command.Commands() {
			wrapCommandTree(child)
		}
	}

	wrapCommandTree(root)
}

func wrapCommandErrorHandlerWithUsage(handler func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	if handler == nil {
		return nil
	}

	return func(command *cobra.Command, args []string) error {
		err := handler(command, args)
		if shouldPrintUsageForMissingPositionalParameter(command, err, args) {
			printCommandUsageOnError(command)
		}
		return err
	}
}

func shouldPrintUsageForMissingPositionalParameter(command *cobra.Command, err error, args []string) bool {
	if err == nil || len(args) != 0 {
		return false
	}
	if !commandDeclaresPositionalParameters(command) {
		return false
	}

	message := strings.TrimSpace(strings.ToLower(err.Error()))
	if message == "" {
		return false
	}

	if faults.IsCategory(err, faults.ValidationError) {
		if strings.HasPrefix(message, "flag ") {
			return false
		}
		if strings.Contains(message, "input is required") {
			return false
		}
		if strings.Contains(message, "interactive terminal is required") {
			return false
		}
		if strings.Contains(message, "value is required") {
			return false
		}
		return strings.Contains(message, " is required")
	}

	return strings.Contains(message, "arg(s)") && strings.Contains(message, "received 0")
}

func commandDeclaresPositionalParameters(command *cobra.Command) bool {
	if command == nil {
		return false
	}

	use := strings.TrimSpace(command.Use)
	return strings.Contains(use, "[") || strings.Contains(use, "<")
}

func printCommandUsageOnError(command *cobra.Command) {
	if command == nil {
		return
	}

	rendered := strings.TrimRight(command.UsageString(), "\n")
	if rendered == "" {
		return
	}

	_, _ = fmt.Fprintln(command.ErrOrStderr(), rendered)
}
