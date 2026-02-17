package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newListCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var source string
	var recursive bool

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List resources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
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

			var items []resource.Resource
			switch source {
			case sourceLocal:
				items, err = orchestratorService.ListLocal(command.Context(), resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			case sourceRemote:
				items, err = orchestratorService.ListRemote(command.Context(), resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			default:
				return common.ValidationError("invalid source: use local or remote", nil)
			}
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.Resource) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVarP(&source, "source", "s", sourceLocal, "list source: local|remote")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "list recursively")
	common.RegisterFlagValueCompletions(command, "source", []string{sourceLocal, sourceRemote})
	return command
}
