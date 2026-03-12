package resource

import (
	"fmt"
	"io"
	"os"
	"strings"

	requestapp "github.com/crmarques/declarest/internal/app/resource/request"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
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
		{method: "patch", short: "Send PATCH request", allowInlinePayload: true},
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
	var payloadInputs []string
	var headerInputs []string
	var acceptType string
	var contentType string
	var yes bool
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

			if cfg.requireDeleteConfirm && !yes {
				return cliutil.ValidationError(
					"flag --yes is required: are you sure you want to delete?",
					nil,
				)
			}
			if !cfg.supportsRecursive && recursive {
				return cliutil.ValidationError("flag --recursive is only supported for resource request delete", nil)
			}

			body, hasBody, err := decodeOptionalRequestPayload(command, contentType, payloadInputs, cfg.allowInlinePayload)
			if err != nil {
				return err
			}
			if !hasBody {
				body = resource.Content{}
			}
			headers, err := parseRequestHeaders(headerInputs)
			if err != nil {
				return err
			}

			result, err := requestapp.Execute(command.Context(), deps, requestapp.Request{
				Method:         methodUpper,
				LogicalPath:    normalizedPath,
				Body:           body,
				Headers:        headers,
				Accept:         acceptType,
				ContentType:    contentType,
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
				return writeRequestOutput(command, deps, globalFlags, resource.Content{})
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
		"payload file path (use '-' for stdin); post/put/patch also accept inline payload; binary requires file or stdin",
	)
	command.Flags().StringArrayVarP(&headerInputs, "header", "H", nil, "request header in 'Name: Value' or 'Name=Value' form")
	command.Flags().StringVar(&acceptType, "accept-type", "", "explicit Accept header override")
	command.Flags().StringVar(&contentType, "content-type", "", "request Content-Type override and payload decode hint")
	cliutil.RegisterResourceInputContentTypeFlagCompletion(command)

	if cfg.requireDeleteConfirm {
		command.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion")
		command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete collection children recursively")
	}
	if isStateChangingRequestMethod(methodUpper) {
		commandmeta.MarkEmitsExecutionStatus(command)
	}

	return command
}

func decodeOptionalRequestPayload(
	command *cobra.Command,
	inputContentType string,
	payloadInputs []string,
	allowInlinePayload bool,
) (resource.Content, bool, error) {
	if len(payloadInputs) > 1 {
		return resource.Content{}, false, cliutil.ValidationError("flag --payload cannot be provided more than once", nil)
	}

	if len(payloadInputs) == 0 {
		data, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{ContentType: inputContentType})
		if err != nil {
			return resource.Content{}, false, err
		}
		if data == nil {
			return resource.Content{}, false, nil
		}
		value, err := cliutil.DecodeResourceContentInputData(data, inputContentType, "")
		if err != nil {
			return resource.Content{}, false, err
		}
		return value, true, nil
	}

	payloadArg := strings.TrimSpace(payloadInputs[0])
	if payloadArg == "" {
		return resource.Content{}, false, cliutil.ValidationError("input is empty", nil)
	}
	if payloadArg == "-" {
		data, err := cliutil.ReadInput(command, cliutil.InputFlags{Payload: "-", ContentType: inputContentType})
		if err != nil {
			return resource.Content{}, false, err
		}
		value, err := cliutil.DecodeResourceContentInputData(data, inputContentType, "")
		if err != nil {
			return resource.Content{}, false, err
		}
		return value, true, nil
	}

	stdinData, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{ContentType: inputContentType})
	if err != nil {
		return resource.Content{}, false, err
	}
	if len(stdinData) > 0 {
		return resource.Content{}, false, cliutil.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if allowInlinePayload && !requestPayloadLooksLikeExistingFile(payloadArg) {
		if cliutil.IsBinaryInputFormat(inputContentType) {
			return resource.Content{}, false, cliutil.ValidationError("binary request payload requires --payload <path|-> or stdin", nil)
		}
		value, err := cliutil.DecodeResourceContentInputData([]byte(payloadArg), inputContentType, "")
		if err != nil {
			return resource.Content{}, false, err
		}
		return value, true, nil
	}

	data, err := cliutil.ReadInput(command, cliutil.InputFlags{Payload: payloadArg, ContentType: inputContentType})
	if err != nil {
		return resource.Content{}, false, err
	}
	value, err := cliutil.DecodeResourceContentInputData(data, inputContentType, payloadArg)
	if err != nil {
		return resource.Content{}, false, err
	}
	return value, true, nil
}

func parseRequestHeaders(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	headers := make(map[string]string, len(values))
	for _, raw := range values {
		name, value, err := splitRequestHeader(raw)
		if err != nil {
			return nil, err
		}
		headers[name] = value
	}
	return headers, nil
}

func splitRequestHeader(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", cliutil.ValidationError("flag --header cannot be empty", nil)
	}

	separator := strings.Index(trimmed, ":")
	if separator < 0 {
		separator = strings.Index(trimmed, "=")
	}
	if separator < 0 {
		return "", "", cliutil.ValidationError("flag --header must use 'Name: Value' or 'Name=Value' syntax", nil)
	}

	name := strings.TrimSpace(trimmed[:separator])
	if name == "" {
		return "", "", cliutil.ValidationError("flag --header requires a header name", nil)
	}
	return name, strings.TrimSpace(trimmed[separator+1:]), nil
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
	outputFormat, err := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, value)
	if err != nil {
		return err
	}

	return cliutil.WriteOutput(command, outputFormat, value, func(w io.Writer, item T) error {
		_, writeErr := fmt.Fprintln(w, item)
		return writeErr
	})
}
