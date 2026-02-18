package resource

import (
	"context"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newUpdateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var recursive bool

	command := &cobra.Command{
		Use:   "update [path]",
		Short: "Update remote resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			targets, err := listLocalMutationTargets(command.Context(), orchestratorService, resolvedPath, recursive)
			if err != nil {
				return err
			}
			items, err := executeMutationForTargets(
				command.Context(),
				targets,
				func(ctx context.Context, logicalPath string) (resource.Resource, error) {
					localValue, getErr := orchestratorService.GetLocal(ctx, logicalPath)
					if getErr != nil {
						return resource.Resource{}, getErr
					}
					return orchestratorService.Update(ctx, logicalPath, localValue)
				},
			)
			if err != nil {
				return err
			}

			return writeCollectionMutationOutput(command, outputFormat, resolvedPath, items)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	return command
}
