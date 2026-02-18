package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
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
