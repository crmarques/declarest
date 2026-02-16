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
	var local bool
	var remote bool

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			if local && remote {
				return common.ValidationError("flags --local and --remote cannot be used together", nil)
			}

			source := sourceRemote
			if local {
				source = sourceLocal
			} else if remote {
				source = sourceRemote
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
			case sourceLocal:
				value, err = orchestratorService.GetLocal(command.Context(), resolvedPath)
			case sourceRemote:
				value, err = orchestratorService.GetRemote(command.Context(), resolvedPath)
			default:
				return common.ValidationError("invalid source: use --local or --remote", nil)
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
	command.Flags().BoolVar(&local, "local", false, "read from local repository")
	command.Flags().BoolVar(&remote, "remote", false, "read from remote server (default)")
	return command
}
