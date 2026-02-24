package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newExplainCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "explain [path]",
		Short: "Explain planned changes",
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
			items, err := orchestratorService.Explain(command.Context(), resolvedPath)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.DiffEntry) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintf(w, "%s %s\n", item.Operation, joinDiffEntryPath(item)); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}
