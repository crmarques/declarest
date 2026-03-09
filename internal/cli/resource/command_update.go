package resource

import (
	"strings"

	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

func newUpdateCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input cliutil.InputFlags
	var recursive bool
	var httpMethod string
	var refresh bool

	command := &cobra.Command{
		Use:   "update [path]",
		Short: "Update remote resource",
		Long: strings.Join([]string{
			"Update remote resources using payloads from the resource repository by default.",
			"When --payload <path|-> or stdin is provided, the explicit payload overrides repository input for a single target path.",
			"This explicit-input mode is useful for direct remote operations when no repository is configured.",
			"Use --refresh to fetch the remote state after each update and persist it locally.",
		}, " "),
		Example: strings.Join([]string{
			"  declarest resource update /customers/acme",
			"  declarest resource update /customers/ --recursive",
			"  declarest resource update /customers/acme --payload payload.json",
			"  cat payload.json | declarest resource update /customers/acme --payload -",
			"  declarest resource update /customers/acme --refresh",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			runCtx, _, err := applyHTTPMethodOverride(command.Context(), httpMethod, metadata.OperationUpdate)
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
				Repository:   deps.Services.RepositoryStore(),
				Metadata:     deps.Services.MetadataService(),
				Secrets:      deps.Services.SecretProvider(),
			}, mutateapp.Request{
				Operation:        mutateapp.OperationUpdate,
				LogicalPath:      mutationPath,
				Recursive:        recursive,
				Value:            value,
				HasExplicitInput: hasExplicitInput,
				RefreshLocal:     refresh,
			})
			if err != nil {
				return err
			}

			if !cliutil.IsVerbose(globalFlags) {
				return nil
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			return writeCollectionMutationOutput(command, outputFormat, result.ResolvedPath, result.Items)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	cliutil.BindResourceInputFlags(command, &input)
	if flag := command.Flags().Lookup("payload"); flag != nil {
		flag.Usage = "payload file path (use '-' to read object from stdin); also accepts inline JSON/YAML, JSON Pointer assignments (/a=b,/c/d=e), or dot-notation assignments (a.b=x,c=y); binary requires file or stdin"
	}
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "walk collection recursively")
	command.Flags().BoolVar(&refresh, "refresh", false, "re-fetch remote mutation results into the repository")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
