package resource

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	resourceinputapp "github.com/crmarques/declarest/internal/app/resource/input"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newApplyCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var recursive bool
	var httpMethod string
	var refreshRepository bool

	command := &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply local desired state (create-or-update remote)",
		Long: strings.Join([]string{
			"Apply desired state from the resource repository by default.",
			"When --payload <path|-> or stdin is provided, the explicit payload overrides repository input for a single target path.",
			"Apply uses upsert behavior for remote writes: create when the resource does not exist, update when it already exists.",
			"This explicit-input mode is useful for direct remote operations when no repository is configured.",
			"Use --refresh-repository to fetch the remote state after each mutation and persist it back into the repository.",
		}, " "),
		Example: strings.Join([]string{
			"  declarest resource apply /customers/acme",
			"  declarest resource apply /customers/ --recursive",
			"  declarest resource apply /customers/acme --payload payload.json",
			"  cat payload.json | declarest resource apply /customers/acme --payload -",
			"  declarest resource apply /customers/acme --refresh-repository",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			runCtx, _, err := applyHTTPMethodOverride(
				command.Context(),
				httpMethod,
				metadata.OperationCreate,
				metadata.OperationUpdate,
			)
			if err != nil {
				return err
			}

			value, hasExplicitInput, err := resourceinputapp.DecodeOptionalMutationPayloadInput(command, input)
			if err != nil {
				return err
			}
			if hasExplicitInput {
				if recursive {
					return common.ValidationError(
						"flag --recursive cannot be combined with explicit input; remove input to apply resources from repository",
						nil,
					)
				}
				mutationPath, err := resolveExplicitMutationPayloadPath(
					command.Context(),
					command.CommandPath(),
					deps,
					resolvedPath,
					value,
				)
				if err != nil {
					return err
				}

				_, getRemoteErr := orchestratorService.GetRemote(runCtx, mutationPath)
				var item resource.Resource
				if getRemoteErr == nil {
					item, err = orchestratorService.Update(runCtx, mutationPath, value)
				} else if isTypedErrorCategory(getRemoteErr, faults.NotFoundError) {
					item, err = orchestratorService.Create(runCtx, mutationPath, value)
				} else {
					return getRemoteErr
				}
				if err != nil {
					return err
				}

				if refreshRepository {
					if err := refreshRepositoryForPaths(runCtx, deps, []resource.Resource{item}); err != nil {
						return err
					}
				}

				if !common.IsVerbose(globalFlags) {
					return nil
				}

				outputFormat, outputErr := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
				if outputErr != nil {
					return outputErr
				}

				return writeCollectionMutationOutput(command, outputFormat, mutationPath, []resource.Resource{item})
			}

			targets, err := listLocalMutationTargets(runCtx, orchestratorService, resolvedPath, recursive)
			if err != nil {
				return err
			}
			items, err := executeMutationForTargets(
				runCtx,
				targets,
				func(ctx context.Context, logicalPath string) (resource.Resource, error) {
					return orchestratorService.Apply(ctx, logicalPath)
				},
			)
			if err != nil {
				return err
			}

			if refreshRepository {
				if err := refreshRepositoryForPaths(runCtx, deps, items); err != nil {
					return err
				}
			}

			if !common.IsVerbose(globalFlags) {
				return nil
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			return writeCollectionMutationOutput(command, outputFormat, resolvedPath, items)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	if flag := command.Flags().Lookup("payload"); flag != nil {
		flag.Usage = "payload file path (use '-' to read object from stdin); also accepts inline JSON/YAML or dotted assignments (a=b,c=d)"
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	command.Flags().BoolVar(&refreshRepository, "refresh-repository", false, "re-fetch remote mutation results into the repository")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
