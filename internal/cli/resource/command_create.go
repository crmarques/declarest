package resource

import (
	"context"
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
	var recursive bool

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

			value, hasExplicitInput, err := decodeCreateInput(command, input, payload)
			if err != nil {
				return err
			}
			if hasExplicitInput {
				if recursive {
					return common.ValidationError(
						"flag --recursive cannot be combined with explicit input; remove input to create resources from repository",
						nil,
					)
				}

				item, createErr := orchestratorService.Create(command.Context(), resolvedPath, value)
				if createErr != nil {
					return createErr
				}

				return common.WriteOutput(command, outputFormat, item, func(w io.Writer, output resource.Resource) error {
					_, writeErr := fmt.Fprintln(w, output.LogicalPath)
					return writeErr
				})
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
					return orchestratorService.Create(ctx, logicalPath, localValue)
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
	common.BindInputFlags(command, &input)
	command.Flags().StringVar(&payload, "payload", "", "inline input payload")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	return command
}

func decodeCreateInput(command *cobra.Command, input common.InputFlags, payload string) (resource.Value, bool, error) {
	if strings.TrimSpace(payload) == "" {
		value, hasInput, err := decodeOptionalResourceInput(command, input)
		return value, hasInput, err
	}
	if input.File != "" {
		return nil, false, common.ValidationError("flag --payload cannot be used with --file", nil)
	}

	stdinData, err := common.ReadOptionalInput(command, common.InputFlags{})
	if err != nil {
		return nil, false, err
	}
	if len(stdinData) > 0 {
		return nil, false, common.ValidationError("flag --payload cannot be used with stdin input", nil)
	}

	value, err := common.DecodeInputData[resource.Value]([]byte(payload), input.Format)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}
