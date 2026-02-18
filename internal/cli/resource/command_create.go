package resource

import (
	"fmt"
	"io"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newCreateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var payload string

	command := &cobra.Command{
		Use:   "create [path]",
		Short: "Create remote resource",
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

			value, err := decodeCreateInput(command, input, payload)
			if err != nil {
				return err
			}

			item, err := orchestratorService.Create(command.Context(), resolvedPath, value)
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
	command.Flags().StringVar(&payload, "payload", "", "inline input payload")
	return command
}

func decodeCreateInput(command *cobra.Command, input common.InputFlags, payload string) (resource.Value, error) {
	if strings.TrimSpace(payload) == "" {
		return common.DecodeInput[resource.Value](command, input)
	}
	if input.File != "" {
		return nil, common.ValidationError("flag --payload cannot be used with --file", nil)
	}

	stdinData, err := common.ReadOptionalInput(command, common.InputFlags{})
	if err != nil {
		return nil, err
	}
	if len(stdinData) > 0 {
		return nil, common.ValidationError("flag --payload cannot be used with stdin input", nil)
	}

	return common.DecodeInputData[resource.Value]([]byte(payload), input.Format)
}
