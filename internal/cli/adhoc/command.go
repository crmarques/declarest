package adhoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
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
	var recursive bool
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

			if method == http.MethodDelete {
				targets, err := listAdHocDeleteTargets(command.Context(), orchestratorService, resolvedPath, recursive)
				if err != nil {
					return err
				}

				values := make([]resource.Value, 0, len(targets))
				for _, target := range targets {
					debugctx.Printf(
						command.Context(),
						"ad-hoc request method=%q path=%q has_body=%t",
						method,
						target.LogicalPath,
						body != nil,
					)

					value, requestErr := orchestratorService.AdHoc(command.Context(), method, target.LogicalPath, body)
					if requestErr != nil {
						debugctx.Printf(
							command.Context(),
							"ad-hoc request failed method=%q path=%q error=%v",
							method,
							target.LogicalPath,
							requestErr,
						)
						return requestErr
					}

					debugctx.Printf(
						command.Context(),
						"ad-hoc request succeeded method=%q path=%q value_type=%T",
						method,
						target.LogicalPath,
						value,
					)
					values = append(values, value)
				}

				if len(values) == 1 && targets[0].LogicalPath == resolvedPath {
					return common.WriteOutput(command, outputFormat, values[0], func(w io.Writer, item resource.Value) error {
						_, writeErr := fmt.Fprintln(w, item)
						return writeErr
					})
				}

				return common.WriteOutput(command, outputFormat, values, func(w io.Writer, items []resource.Value) error {
					for _, item := range items {
						if _, writeErr := fmt.Fprintln(w, item); writeErr != nil {
							return writeErr
						}
					}
					return nil
				})
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
				if method == http.MethodGet && body == nil && isNotFoundError(err) {
					if _, normalizeErr := resource.NormalizeLogicalPath(resolvedPath); normalizeErr == nil {
						fallbackValue, fallbackErr := orchestratorService.GetRemote(command.Context(), resolvedPath)
						if fallbackErr == nil {
							debugctx.Printf(
								command.Context(),
								"ad-hoc request fallback succeeded method=%q path=%q value_type=%T",
								method,
								resolvedPath,
								fallbackValue,
							)
							value = fallbackValue
							err = nil
						} else {
							err = fallbackErr
						}
					}
				}
			}
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
		command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
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

func isNotFoundError(err error) bool {
	var typedErr *faults.TypedError
	if errors.As(err, &typedErr) {
		return typedErr.Category == faults.NotFoundError
	}
	return false
}

func listAdHocDeleteTargets(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	items, err := orchestratorService.ListLocal(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	if err != nil {
		if isNotFoundError(err) {
			return []resource.Resource{{LogicalPath: logicalPath}}, nil
		}
		return nil, err
	}
	if len(items) == 0 {
		return []resource.Resource{{LogicalPath: logicalPath}}, nil
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}
