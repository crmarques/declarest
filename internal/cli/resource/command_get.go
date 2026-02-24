package resource

import (
	"fmt"
	"io"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	readapp "github.com/crmarques/declarest/internal/app/resource/read"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var fromRepository bool
	var fromRemoteServer bool
	var showSecrets bool
	var showMetadata bool
	var httpMethod string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Example: strings.Join([]string{
			"  declarest resource get /customers/acme",
			"  declarest resource get --source repository /customers/acme",
			"  declarest resource get /customers/acme --show-metadata",
			"  declarest resource get /customers/acme --show-secrets",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			requestedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			resolvedPath, err := resource.NormalizeLogicalPath(requestedPath)
			if err != nil {
				return err
			}

			source, err := normalizeReadSourceSelection(sourceFlag, fromRepository, fromRemoteServer)
			if err != nil {
				return err
			}
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && source == sourceRepository {
				return common.ValidationError("flag --http-method requires remote-server source", nil)
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			runCtx := command.Context()
			if source == sourceRemoteServer {
				runCtx, _, err = applyHTTPMethodOverride(runCtx, httpMethod, metadata.OperationGet)
				if err != nil {
					return err
				}
			}

			debugctx.Printf(runCtx, "resource get requested path=%q source=%q", resolvedPath, source)

			result, err := readapp.Execute(runCtx, readapp.Dependencies{
				Orchestrator: deps.Orchestrator,
				Contexts:     deps.Contexts,
				Metadata:     deps.Metadata,
				Secrets:      deps.Secrets,
			}, readapp.Request{
				LogicalPath:              resolvedPath,
				Source:                   source,
				ExplicitCollectionTarget: readapp.HasCollectionTargetMarker(requestedPath),
				ShowSecrets:              showSecrets,
				ShowMetadata:             showMetadata,
				ContextName: func() string {
					if globalFlags == nil {
						return ""
					}
					return globalFlags.Context
				}(),
			})
			if err != nil {
				debugctx.Printf(runCtx, "resource get failed path=%q source=%q error=%v", resolvedPath, source, err)
				return err
			}
			debugctx.Printf(runCtx, "resource get succeeded path=%q value_type=%T source=%q", resolvedPath, result.OutputValue, source)

			return common.WriteOutput(command, outputFormat, result.OutputValue, func(w io.Writer, value any) error {
				if !result.HasTextLines() {
					_, writeErr := fmt.Fprintln(w, value)
					return writeErr
				}
				for _, line := range result.TextLines {
					if _, writeErr := fmt.Fprintln(w, line); writeErr != nil {
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
	bindReadSourceFlags(command, &sourceFlag, &fromRepository, &fromRemoteServer)
	command.Flags().BoolVar(&showSecrets, "show-secrets", false, "show plaintext values for metadata-declared secret attributes")
	command.Flags().BoolVar(&showMetadata, "show-metadata", false, "include rendered metadata snapshot in output")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
