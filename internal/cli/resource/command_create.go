package resource

import (
	"context"
	"strings"

	resourceinputapp "github.com/crmarques/declarest/internal/app/resource/input"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newCreateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var recursive bool
	var httpMethod string
	var refreshRepository bool

	command := &cobra.Command{
		Use:   "create [path]",
		Short: "Create remote resource",
		Long: strings.Join([]string{
			"Create remote resources using payloads from the resource repository by default.",
			"When --payload <path|-> or stdin is provided, the explicit payload overrides repository input for a single target path.",
			"This explicit-input mode is useful for direct remote operations when no repository is configured.",
			"Use --refresh-repository to fetch the remote state after the create and persist it locally.",
		}, " "),
		Example: strings.Join([]string{
			"  declarest resource create /customers/acme",
			"  declarest resource create /customers/ --recursive",
			"  declarest resource create /customers/acme --payload payload.json",
			"  cat payload.json | declarest resource create /customers/acme --payload -",
			"  declarest resource create /customers/acme --refresh-repository",
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
			runCtx, _, err := applyHTTPMethodOverride(command.Context(), httpMethod, metadata.OperationCreate)
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
						"flag --recursive cannot be combined with explicit input; remove input to create resources from repository",
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

				item, createErr := orchestratorService.Create(runCtx, mutationPath, value)
				if createErr != nil {
					return createErr
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
					localValue, getErr := orchestratorService.GetLocal(ctx, logicalPath)
					if getErr != nil {
						return resource.Resource{}, getErr
					}
					return orchestratorService.Create(ctx, logicalPath, localValue)
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
