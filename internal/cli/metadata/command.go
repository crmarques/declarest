package metadata

import (
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandWiring, globalFlags *common.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	command := &cobra.Command{
		Use:   "metadata",
		Short: "Manage metadata",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(),
		newSetCommand(),
		newUnsetCommand(),
		newResolveCommand(),
		newRenderCommand(),
		newInferCommand(),
	)

	return command
}

func newGetCommand() *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := common.ResolvePathInput(pathFlag, args, true); err != nil {
				return err
			}
			return common.NotImplementedError("Metadata", "Get")
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newSetCommand() *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "set [path]",
		Short: "Set metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if _, err := common.ResolvePathInput(pathFlag, args, true); err != nil {
				return err
			}
			if _, err := common.DecodeInput[metadatadomain.ResourceMetadata](command, input); err != nil {
				return err
			}
			return common.NotImplementedError("Metadata", "Set")
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}

func newUnsetCommand() *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "unset [path]",
		Short: "Unset metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := common.ResolvePathInput(pathFlag, args, true); err != nil {
				return err
			}
			return common.NotImplementedError("Metadata", "Unset")
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newResolveCommand() *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "resolve [path]",
		Short: "Resolve metadata for a path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := common.ResolvePathInput(pathFlag, args, true); err != nil {
				return err
			}
			return common.NotImplementedError("Metadata", "Resolve")
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newRenderCommand() *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "render [path] <operation>",
		Short: "Render operation spec",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			pathArgs, operationArg, err := extractRenderArgs(pathFlag, args)
			if err != nil {
				return err
			}
			if _, err := common.ResolvePathInput(pathFlag, pathArgs, true); err != nil {
				return err
			}
			if _, err := parseOperation(operationArg); err != nil {
				return err
			}

			return common.NotImplementedError("Metadata", "Render")
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newInferCommand() *cobra.Command {
	var pathFlag string
	var apply bool
	var recursive bool

	command := &cobra.Command{
		Use:   "infer [path]",
		Short: "Infer metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if _, err := common.ResolvePathInput(pathFlag, args, true); err != nil {
				return err
			}
			_ = metadatadomain.InferenceRequest{Apply: apply, Recursive: recursive}
			return common.NotImplementedError("Metadata", "Infer")
		},
	}

	common.BindPathFlag(command, &pathFlag)
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
