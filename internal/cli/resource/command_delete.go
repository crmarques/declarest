// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	var yes bool
	var recursive bool
	var httpMethod string
	var commitMessage string

	command := &cobra.Command{
		Use:   "delete [path]",
		Short: "Delete a resource",
		Example: strings.Join([]string{
			"  declarest resource delete /customers/acme --yes",
			"  declarest resource delete /customers/ --recursive --yes",
			"  declarest resource delete /customers/acme --source repository --yes",
			"  declarest resource delete /customers/acme --source both --yes",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return cliutil.ValidationError("flag --yes is required: confirm deletion", nil)
			}
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			source, err := normalizeDeleteSourceSelection(sourceFlag)
			if err != nil {
				return err
			}

			deleteFromRemote := source == sourceManagedServer || source == sourceBoth
			deleteFromRepository := source == sourceRepository || source == sourceBoth
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && !deleteFromRemote {
				return cliutil.ValidationError("flag --http-method requires managed-server source", nil)
			}

			var cfg configdomain.Context
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
					commitMessage,
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
	command.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	bindDeleteSourceFlags(command, &sourceFlag)
	bindHTTPMethodFlag(command, &httpMethod)
	bindRepositoryCommitMessageFlags(command, &commitMessage)
	return command
}
