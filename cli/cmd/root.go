package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	noStatusOutput bool
)

var rootCmd = newRootCommand()

const (
	groupUtility    = "utility"
	groupUserFacing = "user"
)

func Execute() error {
	return rootCmd.Execute()
}

func NewRootCommand() *cobra.Command {
	return newRootCommand()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "declarest",
		Short: "Manage declarative resources, contexts, and Git-backed repositories",
		Long: `DeclaREST keeps resource repository definitions and remote managed resources in sync.

Use the CLI to:
  - configure how DeclaREST connects to managed servers and Git repositories
  - pull remote resources into version control
  - apply repository changes back to the remote system, or clean up resources entirely`,
		Example: `  # Pull a remote resource into the repository and persist the file there
  declarest resource save --path /projects/example

  # Apply a repository resource definition to the remote managed server
  declarest resource apply --path /projects/example

  # Inspect and switch contexts before issuing resource commands
  declarest config list
  declarest config use staging`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	cmd.SetHelpCommandGroupID(groupUtility)
	cmd.SetCompletionCommandGroupID(groupUtility)

	configureUsage(cmd)

	cmd.PersistentFlags().BoolVar(&noStatusOutput, "no-status", false, "Suppress status messages and print only command output")
	cmd.PersistentFlags().String("debug", "", "Print grouped debug information (groups: network, repository, resource, all)")
	cmd.PersistentFlags().Lookup("debug").NoOptDefVal = debugGroupAll

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if err == nil {
			return nil
		}
		return usageError(cmd, err.Error())
	})

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return configureDebugSettings(cmd)
	}

	cmd.AddGroup(&cobra.Group{ID: groupUserFacing, Title: "Commands:"})
	cmd.AddGroup(&cobra.Group{ID: groupUtility, Title: "Utility Commands:"})

	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newResourceCommand())
	cmd.AddCommand(newAdHocCommand())
	cmd.AddCommand(newMetadataCommand())
	cmd.AddCommand(newRepoCommand())
	cmd.AddCommand(newSecretCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}
