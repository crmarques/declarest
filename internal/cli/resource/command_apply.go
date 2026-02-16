package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newApplyCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply local desired state",
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
			item, err := orchestratorService.Apply(command.Context(), resolvedPath)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, item, func(w io.Writer, value resource.Resource) error {
				_, writeErr := fmt.Fprintln(w, value.LogicalPath)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}
