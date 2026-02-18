package adhoc

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
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
	var payload string
	var force bool
	methodLower := strings.ToLower(method)

	command := &cobra.Command{
		Use:   methodLower + " [path]",
		Short: fmt.Sprintf("Execute HTTP %s request", method),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if method == http.MethodDelete && !force {
				return common.ValidationError(
					"delete request is destructive: are you sure you want to remove this resource? rerun with --force to confirm",
					nil,
				)
			}

			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			body, err := decodeOptionalBody(command, input, payload)
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
	if method == http.MethodPost || method == http.MethodPut {
		command.Flags().StringVar(&payload, "payload", "", "inline request payload")
	}
	if method == http.MethodDelete {
		command.Flags().BoolVarP(&force, "force", "y", false, "confirm deletion")
	}
	return command
}

func decodeOptionalBody(command *cobra.Command, flags common.InputFlags, payload string) (resource.Value, error) {
	if strings.TrimSpace(payload) != "" {
		if flags.File != "" {
			return nil, common.ValidationError("flag --payload cannot be used with --file", nil)
		}

		stdinData, err := common.ReadOptionalInput(command, common.InputFlags{})
		if err != nil {
			return nil, err
		}
		if len(stdinData) > 0 {
			return nil, common.ValidationError("flag --payload cannot be used with stdin input", nil)
		}

		return common.DecodeInputData[resource.Value]([]byte(payload), flags.Format)
	}

	data, err := common.ReadOptionalInput(command, flags)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	return common.DecodeInputData[resource.Value](data, flags.Format)
}
