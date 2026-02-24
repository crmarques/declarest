package resource

import (
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/spf13/cobra"
)

func newDeleteCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var confirmDelete bool
	var recursive bool
	var fromRepository bool
	var fromRemoteServer bool
	var fromBoth bool
	var httpMethod string

	command := &cobra.Command{
		Use:   "delete [path]",
		Short: "Delete a resource",
		Example: strings.Join([]string{
			"  declarest resource delete /customers/acme --confirm-delete",
			"  declarest resource delete /customers/ --recursive --confirm-delete",
			"  declarest resource delete /customers/acme --source repository --confirm-delete",
			"  declarest resource delete /customers/acme --source both --confirm-delete",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !confirmDelete {
				return common.ValidationError("flag --confirm-delete is required: confirm deletion", nil)
			}
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			source, err := normalizeDeleteSourceSelection(sourceFlag, fromRepository, fromRemoteServer, fromBoth)
			if err != nil {
				return err
			}

			deleteFromRemote := source == sourceRemoteServer || source == sourceBoth
			deleteFromRepository := source == sourceRepository || source == sourceBoth
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && !deleteFromRemote {
				return common.ValidationError("flag --http-method requires remote-server source", nil)
			}

			if deleteFromRemote {
				orchestratorService, err := common.RequireOrchestrator(deps)
				if err != nil {
					return err
				}
				runCtx, _, err := applyHTTPMethodOverride(command.Context(), httpMethod, metadata.OperationDelete)
				if err != nil {
					return err
				}

				targets, err := listLocalMutationTargetsOrFallbackPath(
					runCtx,
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
					if err := orchestratorService.Delete(runCtx, target.LogicalPath, policy); err != nil {
						return err
					}
				}
			}

			if !deleteFromRepository {
				return nil
			}

			repositoryService, err := common.RequireResourceStore(deps)
			if err != nil {
				return err
			}
			return repositoryService.Delete(command.Context(), resolvedPath, repository.DeletePolicy{Recursive: recursive})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&confirmDelete, "confirm-delete", "y", false, "confirm deletion")
	command.Flags().BoolVar(&confirmDelete, "force", false, "legacy alias for --confirm-delete")
	_ = command.Flags().MarkHidden("force")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	bindDeleteSourceFlags(command, &sourceFlag, &fromRepository, &fromRemoteServer, &fromBoth)
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
