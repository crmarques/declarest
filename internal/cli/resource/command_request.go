package resource

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

type requestMethodConfig struct {
	method               string
	short                string
	allowInlinePayload   bool
	requireDeleteConfirm bool
	supportsRecursive    bool
}

func newRequestCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "request",
		Short: "Send raw HTTP requests to the resource server",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	methods := []requestMethodConfig{
		{method: "get", short: "Send GET request"},
		{method: "head", short: "Send HEAD request"},
		{method: "options", short: "Send OPTIONS request"},
		{method: "post", short: "Send POST request", allowInlinePayload: true},
		{method: "put", short: "Send PUT request", allowInlinePayload: true},
		{method: "patch", short: "Send PATCH request"},
		{method: "delete", short: "Send DELETE request", requireDeleteConfirm: true, supportsRecursive: true},
		{method: "trace", short: "Send TRACE request"},
		{method: "connect", short: "Send CONNECT request"},
	}
	for _, method := range methods {
		command.AddCommand(newRequestMethodCommand(deps, globalFlags, method))
	}

	return command
}

func newRequestMethodCommand(
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
	cfg requestMethodConfig,
) *cobra.Command {
	var pathFlag string
	var inputFormat string
	var payloadInputs []string
	var confirmDelete bool
	var recursive bool

	methodUpper := strings.ToUpper(cfg.method)

	command := &cobra.Command{
		Use:   cfg.method + " [path]",
		Short: cfg.short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			normalizedPath, err := resource.NormalizeLogicalPath(resolvedPath)
			if err != nil {
				return err
			}

			if cfg.requireDeleteConfirm && !confirmDelete {
				return common.ValidationError(
					"flag --confirm-delete is required: are you sure you want to delete?",
					nil,
				)
			}
			if !cfg.supportsRecursive && recursive {
				return common.ValidationError("flag --recursive is only supported for resource request delete", nil)
			}

			body, hasBody, err := decodeOptionalRequestPayload(command, inputFormat, payloadInputs, cfg.allowInlinePayload)
			if err != nil {
				return err
			}
			if !hasBody {
				body = nil
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			if cfg.requireDeleteConfirm {
				targets, err := listLocalMutationTargetsOrFallbackPath(
					command.Context(),
					orchestratorService,
					normalizedPath,
					recursive,
				)
				if err != nil {
					return err
				}

				results := make([]resource.Value, 0, len(targets))
				for _, target := range targets {
					value, err := orchestratorService.Request(command.Context(), methodUpper, target.LogicalPath, body)
					if err != nil {
						return err
					}
					results = append(results, value)
				}

				if !common.IsVerbose(globalFlags) {
					return nil
				}
				return writeRequestOutput(command, deps, globalFlags, results)
			}

			value, err := orchestratorService.Request(command.Context(), methodUpper, normalizedPath, body)
			if err != nil {
				return err
			}

			if isStateChangingRequestMethod(methodUpper) && !common.IsVerbose(globalFlags) {
				return nil
			}
			return writeRequestOutput(command, deps, globalFlags, value)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)

	command.Flags().StringArrayVarP(
		&payloadInputs,
		"payload",
		"f",
		nil,
		"payload file path (use '-' for stdin); post/put also accept inline payload",
	)
	command.Flags().StringVarP(&inputFormat, "format", "i", common.OutputJSON, "input format: json|yaml")
	common.RegisterInputFormatFlagCompletion(command)

	if cfg.requireDeleteConfirm {
		command.Flags().BoolVarP(&confirmDelete, "confirm-delete", "y", false, "confirm deletion")
		command.Flags().BoolVar(&confirmDelete, "force", false, "legacy alias for --confirm-delete")
		_ = command.Flags().MarkHidden("force")
		command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete collection children recursively")
	}

	return command
}

func decodeOptionalRequestPayload(
	command *cobra.Command,
	inputFormat string,
	payloadInputs []string,
	allowInlinePayload bool,
) (resource.Value, bool, error) {
	if len(payloadInputs) > 1 {
		return nil, false, common.ValidationError("flag --payload cannot be provided more than once", nil)
	}

	if len(payloadInputs) == 0 {
		data, err := common.ReadOptionalInput(command, common.InputFlags{})
		if err != nil {
			return nil, false, err
		}
		if len(data) == 0 {
			return nil, false, nil
		}
		value, err := common.DecodeInputData[resource.Value](data, inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	payloadArg := strings.TrimSpace(payloadInputs[0])
	if payloadArg == "" {
		return nil, false, common.ValidationError("input is empty", nil)
	}
	if payloadArg == "-" {
		data, err := common.ReadInput(command, common.InputFlags{Payload: "-", Format: inputFormat})
		if err != nil {
			return nil, false, err
		}
		value, err := common.DecodeInputData[resource.Value](data, inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	stdinData, err := common.ReadOptionalInput(command, common.InputFlags{})
	if err != nil {
		return nil, false, err
	}
	if len(stdinData) > 0 {
		return nil, false, common.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if allowInlinePayload && !requestPayloadLooksLikeExistingFile(payloadArg) {
		value, err := common.DecodeInputData[resource.Value]([]byte(payloadArg), inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	data, err := common.ReadInput(command, common.InputFlags{Payload: payloadArg, Format: inputFormat})
	if err != nil {
		return nil, false, err
	}
	value, err := common.DecodeInputData[resource.Value](data, inputFormat)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func requestPayloadLooksLikeExistingFile(value string) bool {
	info, err := os.Stat(value)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func isStateChangingRequestMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE", "CONNECT":
		return true
	default:
		return false
	}
}

func writeRequestOutput[T any](
	command *cobra.Command,
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
	value T,
) error {
	outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
	if err != nil {
		return err
	}

	return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item T) error {
		_, writeErr := fmt.Fprintln(w, item)
		return writeErr
	})
}
