package resource

import (
	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/spf13/cobra"
)

func newDeleteCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var force bool
	var recursive bool
	var fromRepository bool
	var fromRemoteServer bool
	var fromBoth bool

	command := &cobra.Command{
		Use:   "delete [path]",
		Short: "Delete a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !force {
				return common.ValidationError("flag --force is required: confirm deletion", nil)
			}
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			explicitSources := 0
			if fromRepository {
				explicitSources++
			}
			if fromRemoteServer {
				explicitSources++
			}
			if fromBoth {
				explicitSources++
			}
			if explicitSources > 1 {
				return common.ValidationError("flags --repository, --remote-server, and --both are mutually exclusive", nil)
			}

			deleteFromRemote := fromRemoteServer || fromBoth || (!fromRepository && !fromRemoteServer && !fromBoth)
			deleteFromRepository := fromRepository || fromBoth

			if deleteFromRemote {
				orchestratorService, err := common.RequireOrchestrator(deps)
				if err != nil {
					return err
				}

				targets, err := listLocalMutationTargetsOrFallbackPath(
					command.Context(),
					orchestratorService,
					resolvedPath,
					recursive,
				)
				if err != nil {
					return err
				}

				for _, target := range targets {
					policy := orchestratordomain.DeletePolicy{
						Recursive: recursive && target.LogicalPath == resolvedPath,
					}
					if err := orchestratorService.Delete(command.Context(), target.LogicalPath, policy); err != nil {
						return err
					}
				}
			}

			if !deleteFromRepository {
				return nil
			}

			repositoryService, err := common.RequireRepository(deps)
			if err != nil {
				return err
			}
			return repositoryService.Delete(command.Context(), resolvedPath, repository.DeletePolicy{Recursive: recursive})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&force, "force", "y", false, "confirm deletion")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	command.Flags().BoolVar(&fromRepository, "repository", false, "delete from repository")
	command.Flags().BoolVar(&fromRemoteServer, "remote-server", false, "delete from remote server (default)")
	command.Flags().BoolVar(&fromBoth, "both", false, "delete from both remote server and repository")
	return command
}
