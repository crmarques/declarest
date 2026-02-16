package cli

import (
	"context"

	"github.com/crmarques/declarest/internal/cli/adhoc"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/internal/cli/completion"
	"github.com/crmarques/declarest/internal/cli/config"
	metadatacmd "github.com/crmarques/declarest/internal/cli/metadata"
	"github.com/crmarques/declarest/internal/cli/repo"
	resourcecmd "github.com/crmarques/declarest/internal/cli/resource"
	"github.com/crmarques/declarest/internal/cli/secret"
	"github.com/crmarques/declarest/internal/cli/version"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/spf13/cobra"
)

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

			commandContext := context.Background()
			commandContext = common.WithContextName(commandContext, globalFlags.Context)
			commandContext = debugctx.WithEnabled(commandContext, globalFlags.Debug)
			commandContext = debugctx.WithWriter(commandContext, command.ErrOrStderr())
			command.SetContext(commandContext)

			debugctx.Printf(
				command.Context(),
				"root flags context=%q output=%q no_status=%t command=%q",
				globalFlags.Context,
				globalFlags.Output,
				globalFlags.NoStatus,
				command.CommandPath(),
			)

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetHelpCommand(&cobra.Command{
		Use:     "__help",
		Hidden:  true,
		GroupID: "other",
	})

	common.BindGlobalFlags(root, &globalFlags)

	root.AddGroup(
		&cobra.Group{ID: "basic", Title: "Basic Commands:"},
		&cobra.Group{ID: "other", Title: "Other Commands:"},
	)

	basicCommands := []*cobra.Command{
		adhoc.NewCommand(commandDeps, &globalFlags),
		config.NewCommand(commandDeps, &globalFlags),
		metadatacmd.NewCommand(commandDeps, &globalFlags),
		repo.NewCommand(commandDeps, &globalFlags),
		resourcecmd.NewCommand(commandDeps, &globalFlags),
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

	return root
}
