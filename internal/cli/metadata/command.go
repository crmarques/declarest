package metadata

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "metadata",
		Short: "Manage metadata",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(deps, globalFlags),
		newSetCommand(deps),
		newUnsetCommand(deps),
		newResolveCommand(deps, globalFlags),
		newRenderCommand(deps, globalFlags),
		newInferCommand(deps, globalFlags),
	)

	return command
}

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata get requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			item, err := service.Get(command.Context(), resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata get succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func newSetCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "set [path]",
		Short: "Set metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata set requested path=%q", resolvedPath)

			item, err := common.DecodeInput[metadatadomain.ResourceMetadata](command, input)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}

			if err := service.Set(command.Context(), resolvedPath, item); err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}
			debugctx.Printf(command.Context(), "metadata set succeeded path=%q", resolvedPath)
			return nil
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	return command
}

func newUnsetCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "unset [path]",
		Short: "Unset metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata unset requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata unset failed path=%q error=%v", resolvedPath, err)
				return err
			}

			if err := service.Unset(command.Context(), resolvedPath); err != nil {
				debugctx.Printf(command.Context(), "metadata unset failed path=%q error=%v", resolvedPath, err)
				return err
			}
			debugctx.Printf(command.Context(), "metadata unset succeeded path=%q", resolvedPath)
			return nil
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func newResolveCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "resolve [path]",
		Short: "Resolve metadata for a path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata resolve requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			item, err := service.ResolveForPath(command.Context(), resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata resolve succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func newRenderCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "render [path] <operation>",
		Short: "Render operation spec",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(command *cobra.Command, args []string) error {
			pathArgs, operationArg, err := extractRenderArgs(pathFlag, args)
			if err != nil {
				return err
			}

			resolvedPath, err := common.ResolvePathInput(pathFlag, pathArgs, true)
			if err != nil {
				return err
			}

			operation, err := parseOperation(operationArg)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operationArg, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata render requested path=%q operation=%q", resolvedPath, operation)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			item, err := service.RenderOperationSpec(command.Context(), resolvedPath, operation, map[string]any{})
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata render succeeded path=%q operation=%q", resolvedPath, operation)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	operationValues := []string{
		string(metadatadomain.OperationGet),
		string(metadatadomain.OperationCreate),
		string(metadatadomain.OperationUpdate),
		string(metadatadomain.OperationDelete),
		string(metadatadomain.OperationList),
		string(metadatadomain.OperationCompare),
	}
	command.ValidArgsFunction = func(
		command *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if strings.TrimSpace(pathFlag) != "" {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return common.CompleteValues(operationValues, toComplete)
		}

		switch len(args) {
		case 0:
			return common.CompleteLogicalPaths(command, deps, toComplete)
		case 1:
			return common.CompleteValues(operationValues, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return command
}

func newInferCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var apply bool
	var recursive bool

	command := &cobra.Command{
		Use:   "infer [path]",
		Short: "Infer metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(
				command.Context(),
				"metadata infer requested path=%q apply=%t recursive=%t",
				resolvedPath,
				apply,
				recursive,
			)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			request := metadatadomain.InferenceRequest{Apply: apply, Recursive: recursive}
			item, err := service.Infer(command.Context(), resolvedPath, request)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata infer succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&apply, "apply", "a", false, "apply inferred metadata")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "infer recursively")
	return command
}

func parseOperation(value string) (metadatadomain.Operation, error) {
	switch value {
	case string(metadatadomain.OperationGet):
		return metadatadomain.OperationGet, nil
	case string(metadatadomain.OperationCreate):
		return metadatadomain.OperationCreate, nil
	case string(metadatadomain.OperationUpdate):
		return metadatadomain.OperationUpdate, nil
	case string(metadatadomain.OperationDelete):
		return metadatadomain.OperationDelete, nil
	case string(metadatadomain.OperationList):
		return metadatadomain.OperationList, nil
	case string(metadatadomain.OperationCompare):
		return metadatadomain.OperationCompare, nil
	default:
		return "", common.ValidationError("invalid operation", nil)
	}
}

func extractRenderArgs(pathFlag string, args []string) ([]string, string, error) {
	switch len(args) {
	case 1:
		if pathFlag != "" {
			return nil, args[0], nil
		}
		return nil, "", common.ValidationError("operation is required", nil)
	case 2:
		return []string{args[0]}, args[1], nil
	default:
		return nil, "", common.ValidationError("invalid render arguments", nil)
	}
}
