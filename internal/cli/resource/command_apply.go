package resource

import (
	"strings"

	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/crmarques/declarest/internal/cli/shared"
	"github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func newApplyCommand(deps shared.CommandDependencies, globalFlags *shared.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input shared.InputFlags
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
			resolvedPath, err := shared.ResolvePathInput(pathFlag, args, true)
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

			mutationPath := resolvedPath
			if hasExplicitInput {
				mutationPath, err = resolveExplicitMutationPayloadPath(
					command.Context(),
					command.CommandPath(),
					deps,
					resolvedPath,
					value,
				)
				if err != nil {
					return err
				}
			}

			result, err := mutateapp.Execute(runCtx, mutateapp.Dependencies{
				Orchestrator: deps.Orchestrator,
				Repository:   deps.ResourceStore,
				Metadata:     deps.Metadata,
				Secrets:      deps.Secrets,
			}, mutateapp.Request{
				Operation:        mutateapp.OperationApply,
				LogicalPath:      mutationPath,
				Recursive:        recursive,
				Value:            value,
				HasExplicitInput: hasExplicitInput,
				RefreshLocal:     refreshRepository,
			})
			if err != nil {
				return err
			}

			if !shared.IsVerbose(globalFlags) {
				return nil
			}

			outputFormat, err := shared.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			return writeCollectionMutationOutput(command, outputFormat, result.ResolvedPath, result.Items)
		},
	}

	shared.BindPathFlag(command, &pathFlag)
	shared.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = shared.SinglePathArgCompletionFunc(deps)
	shared.BindInputFlags(command, &input)
	if flag := command.Flags().Lookup("payload"); flag != nil {
		flag.Usage = "payload file path (use '-' to read object from stdin); also accepts inline JSON/YAML or dotted assignments (a=b,c=d)"
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	command.Flags().BoolVar(&refreshRepository, "refresh-repository", false, "re-fetch remote mutation results into the repository")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
