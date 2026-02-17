package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newUpdateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

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

			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			item, err := orchestratorService.Update(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, item, func(w io.Writer, output resource.Resource) error {
				_, writeErr := fmt.Fprintln(w, output.LogicalPath)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	return command
}
