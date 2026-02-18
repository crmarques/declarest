package resource

import (
	"errors"
	"fmt"
	"io"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var fromRepository bool
	var fromRemoteServer bool

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			if fromRepository && fromRemoteServer {
				return common.ValidationError("flags --repository and --remote-server cannot be used together", nil)
			}

			source := sourceRemoteServer
			if fromRepository {
				source = sourceRepository
			} else if fromRemoteServer {
				source = sourceRemoteServer
			}

			debugctx.Printf(command.Context(), "resource get requested path=%q source=%q", resolvedPath, source)

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			var value resource.Value
			switch source {
			case sourceRepository:
				value, err = orchestratorService.GetLocal(command.Context(), resolvedPath)
			case sourceRemoteServer:
				value, err = orchestratorService.GetRemote(command.Context(), resolvedPath)
			default:
				return common.ValidationError("invalid source: use --repository or --remote-server", nil)
			}
			if err != nil {
				debugctx.Printf(command.Context(), "resource get failed path=%q source=%q error=%v", resolvedPath, source, err)
				if source == sourceRepository && isNotFoundError(err) {
					debugctx.Printf(command.Context(), "resource get treating %q as collection listing", resolvedPath)
					return renderRepositoryCollection(command, outputFormat, orchestratorService, resolvedPath)
				}
				return err
			}

			debugctx.Printf(command.Context(), "resource get succeeded path=%q value_type=%T source=%q", resolvedPath, value, source)

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&fromRepository, "repository", false, "read from repository")
	command.Flags().BoolVar(&fromRemoteServer, "remote-server", false, "read from remote server (default)")
	return command
}

func isNotFoundError(err error) bool {
	var typedErr *faults.TypedError
	if errors.As(err, &typedErr) {
		return typedErr.Category == faults.NotFoundError
	}
	return false
}

func renderRepositoryCollection(command *cobra.Command, outputFormat string, orchestratorService orchestrator.Orchestrator, logicalPath string) error {
	items, err := orchestratorService.ListLocal(command.Context(), logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return err
	}

	payloads := make([]resource.Value, len(items))
	for idx, item := range items {
		payloads[idx] = item.Payload
	}

	return common.WriteOutput(command, outputFormat, payloads, func(w io.Writer, _ []resource.Value) error {
		for _, item := range items {
			if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}
