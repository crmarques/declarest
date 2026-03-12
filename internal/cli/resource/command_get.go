package resource

import (
	"fmt"
	"io"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	readapp "github.com/crmarques/declarest/internal/app/resource/read"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newGetCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var sourceFlag string
	var excludeItemsFlag []string
	var showSecrets bool
	var showMetadata bool
	var httpMethod string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Example: strings.Join([]string{
			"  declarest resource get /customers/acme",
			"  declarest resource get --source repository /customers/acme",
			"  declarest resource get /admin/realms --exclude master --exclude realm1",
			"  declarest resource get /customers/acme --show-metadata",
			"  declarest resource get /customers/acme --show-secrets",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			requestedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			resolvedPath, err := resource.NormalizeLogicalPath(requestedPath)
			if err != nil {
				return err
			}

			source, err := normalizeReadSourceSelection(sourceFlag)
			if err != nil {
				return err
			}
			excludeItems, err := parseExcludeFlag(command, excludeItemsFlag)
			if err != nil {
				return err
			}
			if _, hasOverride, err := validateHTTPMethodOverride(httpMethod); err != nil {
				return err
			} else if hasOverride && source == sourceRepository {
				return cliutil.ValidationError("flag --http-method requires managed-server source", nil)
			}

			runCtx := command.Context()
			if source == sourceManagedServer {
				runCtx, _, err = applyHTTPMethodOverride(runCtx, httpMethod, metadata.OperationGet)
				if err != nil {
					return err
				}
			}

			debugctx.Printf(runCtx, "resource get requested path=%q source=%q", resolvedPath, source)

			result, err := readapp.Execute(runCtx, cliutil.AppDependencies(deps), readapp.Request{
				LogicalPath:              resolvedPath,
				Source:                   source,
				SkipItems:                excludeItems,
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

			outputFormat, err := cliutil.ResolvePayloadAwareOutputFormat(command.Context(), deps, globalFlags, result.OutputValue)
			if err != nil {
				return err
			}
			return cliutil.WriteOutput(command, outputFormat, result.OutputValue, func(w io.Writer, value any) error {
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

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	bindReadSourceFlags(command, &sourceFlag)
	bindExcludeFlag(command, &excludeItemsFlag)
	command.Flags().BoolVar(&showSecrets, "show-secrets", false, "reveal masked secret values (both attribute-level and whole-resource secrets)")
	command.Flags().BoolVar(&showMetadata, "show-metadata", false, "include rendered metadata snapshot in output")
	bindHTTPMethodFlag(command, &httpMethod)
	return command
}
