package resource

import (
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newSaveCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "save [path]",
		Short: "Save local resource value",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
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

			return orchestratorService.Save(command.Context(), resolvedPath, value)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}
