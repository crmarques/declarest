package metadata

import (
	"context"
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "metadata",
		Short: "Manage metadata",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(deps, globalFlags),
		newSetCommand(deps),
		newUnsetCommand(deps),
		newResolveCommand(deps, globalFlags),
		newRenderCommand(deps, globalFlags),
		newInferCommand(deps, globalFlags),
	)

	return command
}

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var overridesOnly bool

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata get requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			item, err := resolvedMetadataForGet(command.Context(), deps, service, resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}
			if !overridesOnly {
				item = metadatadomain.MergeResourceMetadata(metadatadomain.DefaultResourceMetadata(), item)
			}
			resourceFormat := configdomain.ResourceFormatJSON
			if deps.Contexts != nil {
				selection := configdomain.ContextSelection{}
				if globalFlags != nil {
					selection.Name = globalFlags.Context
				}
				resolvedContext, ctxErr := deps.Contexts.ResolveContext(command.Context(), selection)
				if ctxErr != nil {
					debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, ctxErr)
					return ctxErr
				}
				resourceFormat = resolvedContext.Repository.ResourceFormat
			}
			item, err = metadatadomain.ResolveResourceFormatTemplatesInMetadata(item, resourceFormat)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata get succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&overridesOnly, "overrides-only", false, "print only resolved metadata overrides without default fields")
	return command
}

func newSetCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "set [path]",
		Short: "Set metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata set requested path=%q", resolvedPath)

			item, err := common.DecodeInput[metadatadomain.ResourceMetadata](command, input)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}

			if err := service.Set(command.Context(), resolvedPath, item); err != nil {
				debugctx.Printf(command.Context(), "metadata set failed path=%q error=%v", resolvedPath, err)
				return err
			}
			debugctx.Printf(command.Context(), "metadata set succeeded path=%q", resolvedPath)
			return nil
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	return command
}

func newUnsetCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "unset [path]",
		Short: "Unset metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata unset requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata unset failed path=%q error=%v", resolvedPath, err)
				return err
			}

			if err := service.Unset(command.Context(), resolvedPath); err != nil {
				debugctx.Printf(command.Context(), "metadata unset failed path=%q error=%v", resolvedPath, err)
				return err
			}
			debugctx.Printf(command.Context(), "metadata unset succeeded path=%q", resolvedPath)
			return nil
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func newResolveCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "resolve [path]",
		Short: "Resolve metadata for a path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "metadata resolve requested path=%q", resolvedPath)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			item, err := service.ResolveForPath(command.Context(), resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata resolve failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata resolve succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	return command
}

func newRenderCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "render [path] [operation]",
		Short: "Render operation spec",
		Example: strings.Join([]string{
			"  declarest metadata render /customers/acme get",
			"  declarest metadata render /customers/ list",
			"  declarest metadata render --path /customers/acme update",
		}, "\n"),
		Args: cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			pathArgs, operationArg, err := extractRenderArgs(pathFlag, args)
			if err != nil {
				return err
			}

			resolvedPath, err := common.ResolvePathInput(pathFlag, pathArgs, true)
			if err != nil {
				return err
			}

			operation, err := parseRenderOperation(resolvedPath, operationArg)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operationArg, err)
				return err
			}
			operationDefaulted := strings.TrimSpace(operationArg) == ""

			debugctx.Printf(command.Context(), "metadata render requested path=%q operation=%q", resolvedPath, operation)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			item, err := renderMetadataOperation(command.Context(), service, resolvedPath, operation)
			if err != nil && operationDefaulted &&
				operation == metadatadomain.OperationGet &&
				isOperationPathRequiredError(err, metadatadomain.OperationGet) {
				fallbackItem, fallbackErr := renderMetadataOperation(command.Context(), service, resolvedPath, metadatadomain.OperationList)
				if fallbackErr == nil {
					operation = metadatadomain.OperationList
					item = fallbackItem
					err = nil
				}
			}
			if err != nil {
				debugctx.Printf(command.Context(), "metadata render failed path=%q operation=%q error=%v", resolvedPath, operation, err)
				return err
			}

			debugctx.Printf(command.Context(), "metadata render succeeded path=%q operation=%q", resolvedPath, operation)

			return common.WriteOutput(command, outputFormat, item, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	operationValues := []string{
		string(metadatadomain.OperationGet),
		string(metadatadomain.OperationCreate),
		string(metadatadomain.OperationUpdate),
		string(metadatadomain.OperationDelete),
		string(metadatadomain.OperationList),
		string(metadatadomain.OperationCompare),
	}
	command.ValidArgsFunction = func(
		command *cobra.Command,
		args []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		if strings.TrimSpace(pathFlag) != "" {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return common.CompleteValues(operationValues, toComplete)
		}

		switch len(args) {
		case 0:
			return common.CompleteLogicalPaths(command, deps, toComplete)
		case 1:
			return common.CompleteValues(operationValues, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	return command
}

func newInferCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var apply bool
	var recursive bool

	command := &cobra.Command{
		Use:   "infer [path]",
		Short: "Infer metadata",
		Example: strings.Join([]string{
			"  declarest metadata infer /customers/acme",
			"  declarest metadata infer /customers/acme --apply",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(
				command.Context(),
				"metadata infer requested path=%q apply=%t recursive=%t",
				resolvedPath,
				apply,
				recursive,
			)

			service, err := common.RequireMetadataService(deps)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			request := metadatadomain.InferenceRequest{Apply: apply, Recursive: recursive}
			if request.Recursive {
				return common.ValidationError(
					"metadata infer --recursive is not implemented yet",
					nil,
				)
			}

			_, openAPISpec := resolveOpenAPISpec(command.Context(), deps)

			var existingMetadata *metadatadomain.ResourceMetadata
			existing, err := service.Get(command.Context(), resolvedPath)
			if err != nil {
				if !isTypedErrorCategory(err, faults.NotFoundError) {
					debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
					return err
				}
			} else {
				existingMetadata = &existing
			}

			outputItem, err := inferCompactedMetadata(
				command.Context(),
				resolvedPath,
				request,
				openAPISpec,
				existingMetadata,
			)
			if err != nil {
				debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
				return err
			}

			if request.Apply {
				if err := service.Set(command.Context(), resolvedPath, outputItem); err != nil {
					debugctx.Printf(command.Context(), "metadata infer failed path=%q error=%v", resolvedPath, err)
					return err
				}
			}

			debugctx.Printf(command.Context(), "metadata infer succeeded path=%q", resolvedPath)

			return common.WriteOutput(command, outputFormat, outputItem, nil)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&apply, "apply", "a", false, "apply inferred metadata")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "infer recursively")
	_ = command.Flags().MarkHidden("recursive")
	return command
}

func parseOperation(value string) (metadatadomain.Operation, error) {
	switch value {
	case string(metadatadomain.OperationGet):
		return metadatadomain.OperationGet, nil
	case string(metadatadomain.OperationCreate):
		return metadatadomain.OperationCreate, nil
	case string(metadatadomain.OperationUpdate):
		return metadatadomain.OperationUpdate, nil
	case string(metadatadomain.OperationDelete):
		return metadatadomain.OperationDelete, nil
	case string(metadatadomain.OperationList):
		return metadatadomain.OperationList, nil
	case string(metadatadomain.OperationCompare):
		return metadatadomain.OperationCompare, nil
	default:
		return "", common.ValidationError("invalid operation", nil)
	}
}

func parseRenderOperation(logicalPath string, rawOperation string) (metadatadomain.Operation, error) {
	trimmedOperation := strings.TrimSpace(rawOperation)
	if trimmedOperation != "" {
		return parseOperation(trimmedOperation)
	}

	if metadataPathLooksCollection(logicalPath) {
		return metadatadomain.OperationList, nil
	}
	return metadatadomain.OperationGet, nil
}

func renderMetadataOperation(
	ctx context.Context,
	service metadatadomain.MetadataService,
	logicalPath string,
	operation metadatadomain.Operation,
) (metadatadomain.OperationSpec, error) {
	if !metadataPathNeedsSelectorMode(logicalPath) {
		return service.RenderOperationSpec(ctx, logicalPath, operation, map[string]any{})
	}

	metadataValue, err := service.Get(ctx, logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	return resolveOperationSpecWithoutRendering(metadataValue, operation)
}

func resolveOperationSpecWithoutRendering(
	metadataValue metadatadomain.ResourceMetadata,
	operation metadatadomain.Operation,
) (metadatadomain.OperationSpec, error) {
	spec := metadatadomain.OperationSpec{
		Filter:   cloneStringSlice(metadataValue.Filter),
		Suppress: cloneStringSlice(metadataValue.Suppress),
		JQ:       metadataValue.JQ,
	}
	if metadataValue.Operations != nil {
		if operationSpec, found := metadataValue.Operations[string(operation)]; found {
			spec = metadatadomain.MergeOperationSpec(spec, operationSpec)
		}
	}
	if strings.TrimSpace(spec.Path) == "" {
		spec.Path = defaultOperationPathTemplate(operation)
	}

	if strings.TrimSpace(spec.Path) == "" {
		return metadatadomain.OperationSpec{}, common.ValidationError(
			fmt.Sprintf("metadata operation %q path is required", operation),
			nil,
		)
	}
	return spec, nil
}

func extractRenderArgs(pathFlag string, args []string) ([]string, string, error) {
	switch len(args) {
	case 0:
		if pathFlag != "" {
			return nil, "", nil
		}
		return nil, "", common.ValidationError("path is required", nil)
	case 1:
		if pathFlag != "" {
			return nil, args[0], nil
		}
		if _, err := parseOperation(args[0]); err == nil {
			return nil, "", common.ValidationError("path is required", nil)
		}
		return []string{args[0]}, "", nil
	case 2:
		return []string{args[0]}, args[1], nil
	default:
		return nil, "", common.ValidationError("invalid render arguments", nil)
	}
}

func metadataPathNeedsSelectorMode(logicalPath string) bool {
	descriptor, err := metadatadomain.ParsePathDescriptor(logicalPath)
	if err != nil {
		return false
	}
	return descriptor.SelectorMode
}

func metadataPathLooksCollection(logicalPath string) bool {
	descriptor, err := metadatadomain.ParsePathDescriptor(logicalPath)
	if err != nil {
		return false
	}
	return descriptor.Collection
}

func defaultOperationPathTemplate(operation metadatadomain.Operation) string {
	switch operation {
	case metadatadomain.OperationCreate, metadatadomain.OperationList:
		return "."
	default:
		return "./{{.id}}"
	}
}

func inferMetadataFromAvailableEndpoints(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
) (metadatadomain.ResourceMetadata, bool, error) {
	orchestratorService, openAPISpec := resolveOpenAPISpec(ctx, deps)

	existsInOpenAPI, err := metadatadomain.HasOpenAPIPath(logicalPath, openAPISpec)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, false, err
	}

	existsRemotely := false
	if !existsInOpenAPI && orchestratorService != nil {
		existsRemotely, err = metadataPathExistsRemotely(ctx, orchestratorService, logicalPath)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, false, err
		}
	}

	if !existsInOpenAPI && !existsRemotely {
		return metadatadomain.ResourceMetadata{}, false, nil
	}

	compact, err := inferCompactedMetadata(ctx, logicalPath, metadatadomain.InferenceRequest{}, openAPISpec, nil)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, false, err
	}

	return compact, true, nil
}

func resolvedMetadataForGet(
	ctx context.Context,
	deps common.CommandDependencies,
	service metadatadomain.MetadataService,
	logicalPath string,
) (metadatadomain.ResourceMetadata, error) {
	if metadataPathContainsReservedSegment(logicalPath) {
		explicit, err := service.Get(ctx, logicalPath)
		if err == nil {
			return explicit, nil
		}
		if !isTypedErrorCategory(err, faults.NotFoundError) {
			return metadatadomain.ResourceMetadata{}, err
		}

		inferred, ok, inferErr := inferMetadataFromAvailableEndpoints(ctx, deps, logicalPath)
		if inferErr != nil {
			return metadatadomain.ResourceMetadata{}, inferErr
		}
		if ok {
			return inferred, nil
		}
		return metadatadomain.ResourceMetadata{}, err
	}

	resolved, err := service.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	if metadataHasOverrides(resolved) {
		return resolved, nil
	}

	// Preserve explicit empty metadata files (for example `operationInfo: {}`) as
	// a valid hit instead of forcing fallback inference.
	explicit, err := service.Get(ctx, logicalPath)
	if err == nil {
		return explicit, nil
	}
	if !isTypedErrorCategory(err, faults.NotFoundError) {
		return metadatadomain.ResourceMetadata{}, err
	}

	inferred, ok, inferErr := inferMetadataFromAvailableEndpoints(ctx, deps, logicalPath)
	if inferErr != nil {
		return metadatadomain.ResourceMetadata{}, inferErr
	}
	if ok {
		return inferred, nil
	}
	return metadatadomain.ResourceMetadata{}, err
}

func metadataPathContainsReservedSegment(logicalPath string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(logicalPath), "\\", "/")
	if normalized == "/_" {
		return true
	}
	return strings.HasSuffix(normalized, "/_") || strings.Contains(normalized, "/_/")
}

func metadataHasOverrides(item metadatadomain.ResourceMetadata) bool {
	return strings.TrimSpace(item.IDFromAttribute) != "" ||
		strings.TrimSpace(item.AliasFromAttribute) != "" ||
		strings.TrimSpace(item.CollectionPath) != "" ||
		item.SecretsFromAttributes != nil ||
		item.Operations != nil ||
		item.Filter != nil ||
		item.Suppress != nil ||
		strings.TrimSpace(item.JQ) != ""
}

func resolveOpenAPISpec(
	ctx context.Context,
	deps common.CommandDependencies,
) (metadataPathProbe, resource.Value) {
	orchestratorService, err := common.RequireCompletionService(deps)
	if err != nil {
		return nil, nil
	}

	openAPISpec, _ := orchestratorService.GetOpenAPISpec(ctx)
	return orchestratorService, openAPISpec
}

type metadataPathProbe interface {
	orchestratordomain.OpenAPISpecReader
	orchestratordomain.RemoteReader
}

func inferCompactedMetadata(
	ctx context.Context,
	logicalPath string,
	request metadatadomain.InferenceRequest,
	openAPISpec resource.Value,
	existing *metadatadomain.ResourceMetadata,
) (metadatadomain.ResourceMetadata, error) {
	inferred, err := metadatadomain.InferFromOpenAPISpec(ctx, logicalPath, request, openAPISpec)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	if existing != nil {
		inferred = metadatadomain.MergeResourceMetadata(inferred, *existing)
	}

	compact, err := metadatadomain.CompactInferredMetadataDefaults(logicalPath, inferred, openAPISpec)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	return compact, nil
}

func metadataPathExistsRemotely(
	ctx context.Context,
	orchestratorService orchestratordomain.RemoteReader,
	logicalPath string,
) (bool, error) {
	if metadataPathLooksCollection(logicalPath) {
		_, err := orchestratorService.ListRemote(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: false})
		if err == nil {
			return true, nil
		}
		if isTypedErrorCategory(err, faults.NotFoundError) {
			return false, nil
		}
		return false, err
	}

	_, err := orchestratorService.GetRemote(ctx, logicalPath)
	if err == nil {
		return true, nil
	}
	if isTypedErrorCategory(err, faults.NotFoundError) {
		return false, nil
	}
	return false, err
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	items := make([]string, len(values))
	copy(items, values)
	return items
}

func isTypedErrorCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}

func isOperationPathRequiredError(err error, operation metadatadomain.Operation) bool {
	if !isTypedErrorCategory(err, faults.ValidationError) {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, strings.ToLower(fmt.Sprintf("metadata operation %q path is required", operation))) {
		return true
	}

	// Default get paths resolve to "./{{.id}}". When "id" is unavailable in
	// render mode, retry list just like the missing-path fallback.
	return operation == metadatadomain.OperationGet &&
		strings.Contains(message, "failed to render metadata template for path") &&
		strings.Contains(message, "map has no entry for key \"id\"")
}
