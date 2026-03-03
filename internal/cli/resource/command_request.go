package resource

import (
	"fmt"
	"io"
	"os"
	"strings"

	requestapp "github.com/crmarques/declarest/internal/app/resource/request"
	"github.com/crmarques/declarest/internal/cli/cliutil"
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

func newRequestCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "request",
		Short: "Send raw HTTP requests to the managed server",
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
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
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
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			normalizedPath, err := resource.NormalizeLogicalPath(resolvedPath)
			if err != nil {
				return err
			}

			if cfg.requireDeleteConfirm && !confirmDelete {
				return cliutil.ValidationError(
					"flag --confirm-delete is required: are you sure you want to delete?",
					nil,
				)
			}
			if !cfg.supportsRecursive && recursive {
				return cliutil.ValidationError("flag --recursive is only supported for resource request delete", nil)
			}

			body, hasBody, err := decodeOptionalRequestPayload(command, inputFormat, payloadInputs, cfg.allowInlinePayload)
			if err != nil {
				return err
			}
			if !hasBody {
				body = nil
			}

			result, err := requestapp.Execute(command.Context(), requestapp.Dependencies{
				Orchestrator: deps.Orchestrator,
			}, requestapp.Request{
				Method:         methodUpper,
				LogicalPath:    normalizedPath,
				Body:           body,
				ResolveTargets: cfg.requireDeleteConfirm,
				Recursive:      recursive,
			})
			if err != nil {
				return err
			}

			if cfg.requireDeleteConfirm {
				if !cliutil.IsVerbose(globalFlags) {
					return nil
				}
				return writeRequestOutput(command, deps, globalFlags, result.Values)
			}

			if isStateChangingRequestMethod(methodUpper) && !cliutil.IsVerbose(globalFlags) {
				return nil
			}
			if len(result.Values) == 0 {
				return writeRequestOutput(command, deps, globalFlags, resource.Value(nil))
			}
			return writeRequestOutput(command, deps, globalFlags, result.Values[0])
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)

	command.Flags().StringArrayVarP(
		&payloadInputs,
		"payload",
		"f",
		nil,
		"payload file path (use '-' for stdin); post/put also accept inline payload",
	)
	command.Flags().StringVarP(&inputFormat, "format", "i", cliutil.OutputJSON, "input format: json|yaml")
	cliutil.RegisterInputFormatFlagCompletion(command)

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
		return nil, false, cliutil.ValidationError("flag --payload cannot be provided more than once", nil)
	}

	if len(payloadInputs) == 0 {
		data, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{})
		if err != nil {
			return nil, false, err
		}
		if len(data) == 0 {
			return nil, false, nil
		}
		value, err := cliutil.DecodeInputData[resource.Value](data, inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	payloadArg := strings.TrimSpace(payloadInputs[0])
	if payloadArg == "" {
		return nil, false, cliutil.ValidationError("input is empty", nil)
	}
	if payloadArg == "-" {
		data, err := cliutil.ReadInput(command, cliutil.InputFlags{Payload: "-", Format: inputFormat})
		if err != nil {
			return nil, false, err
		}
		value, err := cliutil.DecodeInputData[resource.Value](data, inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	stdinData, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{})
	if err != nil {
		return nil, false, err
	}
	if len(stdinData) > 0 {
		return nil, false, cliutil.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if allowInlinePayload && !requestPayloadLooksLikeExistingFile(payloadArg) {
		value, err := cliutil.DecodeInputData[resource.Value]([]byte(payloadArg), inputFormat)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	}

	data, err := cliutil.ReadInput(command, cliutil.InputFlags{Payload: payloadArg, Format: inputFormat})
	if err != nil {
		return nil, false, err
	}
	value, err := cliutil.DecodeInputData[resource.Value](data, inputFormat)
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
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	value T,
) error {
	outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
	if err != nil {
		return err
	}

	return cliutil.WriteOutput(command, outputFormat, value, func(w io.Writer, item T) error {
		_, writeErr := fmt.Fprintln(w, item)
		return writeErr
	})
}
