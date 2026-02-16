package resource

import (
	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/spf13/cobra"
)

func newDeleteCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var force bool
	var recursive bool

	command := &cobra.Command{
		Use:   "delete [path]",
		Short: "Delete a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !force {
				return command.Help()
			}
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			return orchestratorService.Delete(command.Context(), resolvedPath, orchestratordomain.DeletePolicy{Recursive: recursive})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	command.Flags().BoolVarP(&force, "force", "y", false, "confirm deletion")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	return command
}
