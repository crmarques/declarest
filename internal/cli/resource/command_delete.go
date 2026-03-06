package resource

import (
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/spf13/cobra"
)

func newDeleteCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var confirmDelete bool
	var recursive bool
	var httpMethod string
	var commitMessageAppend string
	var commitMessageOverride string

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
				return cliutil.ValidationError("flag --confirm-delete is required: confirm deletion", nil)
			}
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			source, err := normalizeDeleteSourceSelection(sourceFlag)
			if err != nil {
				return err
			}

			deleteFromRemote := source == sourceRemoteServer || source == sourceBoth
			deleteFromRepository := source == sourceRepository || source == sourceBoth
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && !deleteFromRemote {
				return cliutil.ValidationError("flag --http-method requires remote-server source", nil)
			}

			var cfg configdomain.Context
			var commitMessage string
			if deleteFromRepository {
				cfg, err = resolveActiveResourceContext(command.Context(), deps, nil)
				if err != nil {
					return err
				}
				if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource delete"); err != nil {
					return err
				}
				commitMessage, err = resolveRepositoryCommitMessage(
					command,
					fmt.Sprintf("declarest: delete resource %s", resolvedPath),
					commitMessageAppend,
					commitMessageOverride,
				)
				if err != nil {
					return err
				}
			}

			if deleteFromRemote {
				orchestratorService, err := cliutil.RequireOrchestrator(deps)
				if err != nil {
					return err
				}
				runCtx, _, err := applyHTTPMethodOverride(command.Context(), httpMethod, metadata.OperationDelete)
				if err != nil {
					return err
				}

				targets, err := mutateapp.ListLocalTargetsOrFallbackPath(
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

			repositoryService, err := cliutil.RequireResourceStore(deps)
			if err != nil {
				return err
			}
			if err := repositoryService.Delete(command.Context(), resolvedPath, repository.DeletePolicy{Recursive: recursive}); err != nil {
				return err
			}
			return commitRepositoryIfGit(command.Context(), deps, cfg, commitMessage)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&confirmDelete, "confirm-delete", "y", false, "confirm deletion")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	bindDeleteSourceFlags(command, &sourceFlag)
	bindHTTPMethodFlag(command, &httpMethod)
	bindRepositoryCommitMessageFlags(command, &commitMessageAppend, &commitMessageOverride)
	return command
}
