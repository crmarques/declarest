package adhoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

var supportedHTTPMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodTrace,
	http.MethodConnect,
}

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "ad-hoc",
		Short: "Execute ad-hoc HTTP requests against managed server API",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	for _, method := range supportedHTTPMethods {
		command.AddCommand(newMethodCommand(method, deps, globalFlags))
	}

	return command
}

func newMethodCommand(method string, deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	methodLower := strings.ToLower(method)

	command := &cobra.Command{
		Use:   methodLower + " [path]",
		Short: fmt.Sprintf("Execute HTTP %s request", method),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			body, err := decodeOptionalBody(command, input)
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

			debugctx.Printf(
				command.Context(),
				"ad-hoc request method=%q path=%q has_body=%t",
				method,
				resolvedPath,
				body != nil,
			)

			value, err := orchestratorService.AdHoc(command.Context(), method, resolvedPath, body)
			if err != nil {
				debugctx.Printf(
					command.Context(),
					"ad-hoc request failed method=%q path=%q error=%v",
					method,
					resolvedPath,
					err,
				)
				return err
			}

			debugctx.Printf(
				command.Context(),
				"ad-hoc request succeeded method=%q path=%q value_type=%T",
				method,
				resolvedPath,
				value,
			)

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
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

func decodeOptionalBody(command *cobra.Command, flags common.InputFlags) (resource.Value, error) {
	data, err := readOptionalInput(command, flags)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	var value resource.Value
	switch flags.Format {
	case "", common.OutputJSON:
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, common.ValidationError("invalid json input", err)
		}
	case common.OutputYAML:
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, common.ValidationError("invalid yaml input", err)
		}
	default:
		return nil, common.ValidationError("invalid input format: use json or yaml", nil)
	}

	return value, nil
}

func readOptionalInput(command *cobra.Command, flags common.InputFlags) ([]byte, error) {
	if flags.File != "" {
		data, err := os.ReadFile(flags.File)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, common.ValidationError("input is empty", nil)
		}
		return data, nil
	}

	inputReader := command.InOrStdin()
	if stdinFile, ok := inputReader.(*os.File); ok {
		info, err := stdinFile.Stat()
		if err == nil && (info.Mode()&os.ModeCharDevice) != 0 {
			return nil, nil
		}
	}

	data, err := io.ReadAll(inputReader)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	return data, nil
}
