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
	"github.com/spf13/cobra"
)

func NewRootCommand(deps common.CommandWiring) *cobra.Command {
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
			command.SetContext(common.WithContextName(context.Background(), globalFlags.Context))
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
		adhoc.NewCommand(deps, &globalFlags),
		config.NewCommand(deps, &globalFlags),
		metadatacmd.NewCommand(deps, &globalFlags),
		repo.NewCommand(deps, &globalFlags),
		resourcecmd.NewCommand(deps, &globalFlags),
		secret.NewCommand(deps, &globalFlags),
	}
	for _, command := range basicCommands {
		command.GroupID = "basic"
		root.AddCommand(command)
	}

	otherCommands := []*cobra.Command{
		completion.NewCommand(deps, &globalFlags),
		version.NewCommand(deps, &globalFlags),
	}
	for _, command := range otherCommands {
		command.GroupID = "other"
		root.AddCommand(command)
	}

	return root
}
