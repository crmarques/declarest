package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"declarest/internal/managedserver"
	"declarest/internal/openapi"
	"declarest/internal/reconciler"
	"declarest/internal/resource"
	"declarest/internal/secrets"

	"github.com/spf13/cobra"
)

func newResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resource",
		GroupID: groupUserFacing,
		Short:   "Operate on resources stored in the resource repository",
	}

	cmd.AddCommand(newResourceGetCommand())
	cmd.AddCommand(newResourceSaveCommand())
	cmd.AddCommand(newResourceExplainCommand())
	cmd.AddCommand(newResourceTemplateCommand())
	cmd.AddCommand(newResourceAddCommand())
	cmd.AddCommand(newResourceCreateCommand())
	cmd.AddCommand(newResourceUpdateCommand())
	cmd.AddCommand(newResourceApplyCommand())
	cmd.AddCommand(newResourceDeleteCommand())
	cmd.AddCommand(newResourceDiffCommand())
	cmd.AddCommand(newResourceListCommand())

	return cmd
}

func newResourceGetCommand() *cobra.Command {
	var (
		path        string
		print       bool
		fromRepo    bool
		withSecrets bool
	)

	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Fetch a resource from the remote server (or repository)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			var res resource.Resource
			if fromRepo {
				res, err = recon.GetLocalResource(path)
			} else {
				res, err = recon.GetRemoteResource(path)
			}
			if err != nil {
				if fromRepo {
					return err
				}
				return wrapRemoteErrorWithDetails(err, path)
			}

			secretPaths, err := recon.SecretPathsFor(path)
			if err != nil {
				return err
			}
			warnUnmappedSecrets(cmd, path, res, secretPaths)

			output := res
			if withSecrets {
				if fromRepo {
					secretPaths, err := secretPathsFor(recon, path)
					if err != nil {
						return err
					}
					hasPlaceholders, err := secrets.HasSecretPlaceholders(res, secretPaths)
					if err != nil {
						return err
					}
					if hasPlaceholders {
						output, err = recon.ResolveResourceSecrets(path, res)
						if err != nil {
							return err
						}
					}
				}
			} else {
				output, err = recon.MaskResourceSecrets(path, res, false)
				if err != nil {
					return err
				}
			}

			if print {
				if err := printResourceJSON(cmd, output); err != nil {
					return err
				}
			}

			if fromRepo {
				successf(cmd, "loaded resource %s from the repository", path)
			} else {
				successf(cmd, "fetched remote resource %s", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to read")
	cmd.Flags().BoolVar(&print, "print", true, "Print the resource payload to stdout")
	cmd.Flags().BoolVar(&fromRepo, "repo", false, "Read the resource from the resource repository")
	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include secrets in output (resolves repo placeholders via the secret store)")

	registerResourcePathCompletion(cmd, resourceGetPathStrategy)

	return cmd
}

func newResourceSaveCommand() *cobra.Command {
	var (
		path          string
		print         bool
		withSecrets   bool
		asOneResource bool
		force         bool
	)

	cmd := &cobra.Command{
		Use:   "save <path>",
		Short: "Fetch a remote resource and persist it in the resource repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if asOneResource && !resource.IsCollectionPath(path) {
				return usageError(cmd, "--as-one-resource requires a collection path")
			}
			if withSecrets && !force {
				return fmt.Errorf("refusing to save plaintext secrets without --force (saving secrets in the repository has security implications)")
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			res, err := recon.GetRemoteResource(path)
			if err != nil {
				return wrapRemoteErrorWithDetails(err, path)
			}

			secretPaths, err := recon.SecretPathsFor(path)
			if err != nil {
				return err
			}
			warnUnmappedSecrets(cmd, path, res, secretPaths)

			output := res
			if !withSecrets {
				output, err = recon.MaskResourceSecrets(path, res, false)
				if err != nil {
					return err
				}
			}

			if print {
				if err := printResourceJSON(cmd, output); err != nil {
					return err
				}
			}

			storeSecrets := !withSecrets

			if resource.IsCollectionPath(path) {
				if asOneResource {
					if err := ensureRepositoryOverwriteAllowed(recon, path, force); err != nil {
						return err
					}
					if err := saveLocalResourceWithSecrets(recon, path, res, storeSecrets); err != nil {
						return err
					}
					successf(cmd, "fetched remote collection %s and saved in the resource repository", path)
					return nil
				}

				items, ok := res.AsArray()
				if !ok {
					return usageError(cmd, "collection paths require a collection payload; use --as-one-resource to save the full response")
				}
				var resources []resource.Resource
				for _, item := range items {
					r, err := resource.NewResource(item)
					if err != nil {
						return err
					}
					resources = append(resources, r)
				}
				if err := saveLocalCollectionItemsWithSecrets(recon, path, resources, storeSecrets); err != nil {
					return err
				}
				successf(cmd, "fetched remote collection %s and saved %d items in the resource repository", path, len(resources))
				return nil
			}

			if err := ensureRepositoryOverwriteAllowed(recon, path, force); err != nil {
				return err
			}
			if err := saveLocalResourceWithSecrets(recon, path, res, storeSecrets); err != nil {
				return err
			}
			successf(cmd, "fetched remote resource %s and saved in the resource repository", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to save")
	cmd.Flags().BoolVar(&print, "print", false, "Print the resource payload to stdout")
	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include secrets in output (saving plaintext secrets requires --force)")
	cmd.Flags().BoolVar(&asOneResource, "as-one-resource", false, "Save a fetched collection as a single resource repository entry")
	cmd.Flags().BoolVar(&force, "force", false, "Allow saving plaintext secrets or overriding existing definitions in the resource repository")

	registerResourcePathCompletion(cmd, resourceRemotePathStrategy)

	return cmd
}

func newResourceExplainCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "explain <path>",
		Short: "Explain how metadata/OpenAPI map a logical path to HTTP operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			return runResourceExplain(cmd, path)
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Logical resource or collection path to explain")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newResourceTemplateCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "template <path>",
		Short: "Print a sample payload from the collection's OpenAPI schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if !resource.IsCollectionPath(path) {
				return usageError(cmd, "collection path (ending with /) is required")
			}
			return runResourceTemplate(cmd, path)
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Collection path to sample")

	registerResourcePathCompletion(cmd, resourceRemotePathStrategy)

	return cmd
}

func runResourceExplain(cmd *cobra.Command, path string) error {
	recon, cleanup, err := loadDefaultReconciler()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	if recon.ResourceRecordProvider == nil {
		return errors.New("resource record provider is not configured")
	}

	record, err := recon.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return err
	}

	spec := openapiSpecFromProvider(recon.ResourceRecordProvider)

	return printExplain(cmd.OutOrStdout(), path, record, spec)
}

func runResourceTemplate(cmd *cobra.Command, path string) error {
	recon, cleanup, err := loadDefaultReconciler()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	if recon.ResourceRecordProvider == nil {
		return errors.New("resource record provider is not configured")
	}

	spec := openapiSpecFromProvider(recon.ResourceRecordProvider)
	if spec == nil {
		return errors.New("openapi spec is not configured; provide --spec or configure managed_server.http.openapi")
	}

	normalized := resource.NormalizePath(path)
	schema := openapi.CollectionRequestSchema(spec, normalized)
	if schema == nil {
		return fmt.Errorf("no OpenAPI schema matches %s", normalized)
	}
	value, ok := openapi.DefaultValueForSchema(spec, schema)
	if !ok {
		return fmt.Errorf("OpenAPI schema for %s does not define a sample", normalized)
	}

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if _, err := cmd.OutOrStdout().Write(payload); err != nil {
		return err
	}

	return nil
}

func printExplain(out io.Writer, path string, record resource.ResourceRecord, spec *openapi.Spec) error {
	isCollection := resource.IsCollectionPath(path)
	fmt.Fprintf(out, "Logical Path: %s\n", path)
	fmt.Fprintf(out, "Collection Path: %s\n", record.CollectionPath())
	if info := record.Meta.ResourceInfo; info != nil {
		if attr := strings.TrimSpace(info.IDFromAttribute); attr != "" {
			fmt.Fprintf(out, "ID Attribute: %s\n", attr)
		}
		if attr := strings.TrimSpace(info.AliasFromAttribute); attr != "" {
			fmt.Fprintf(out, "Alias Attribute: %s\n", attr)
		}
		if len(info.SecretInAttributes) > 0 {
			fmt.Fprintf(out, "Secret Attributes: %s\n", strings.Join(info.SecretInAttributes, ", "))
		}
	}
	fmt.Fprintln(out, "\nMetadata Operations:")

	if record.Meta.OperationInfo == nil {
		fmt.Fprintln(out, "  (no metadata operations configured)")
	} else {
		ops := []struct {
			label string
			op    *resource.OperationMetadata
		}{
			{"read", record.Meta.OperationInfo.GetResource},
			{"list", record.Meta.OperationInfo.ListCollection},
			{"create", record.Meta.OperationInfo.CreateResource},
			{"update", record.Meta.OperationInfo.UpdateResource},
			{"delete", record.Meta.OperationInfo.DeleteResource},
		}
		for _, entry := range ops {
			if entry.op == nil {
				continue
			}
			printOperation(entry.label, record, path, isCollection, entry.op, out)
		}
	}

	fmt.Fprintln(out, "\nOpenAPI Metadata:")
	describeOpenAPI(out, spec, record, path, isCollection)

	return nil
}

func printOperation(label string, record resource.ResourceRecord, path string, isCollection bool, op *resource.OperationMetadata, out io.Writer) {
	method := strings.ToUpper(strings.TrimSpace(op.HTTPMethod))
	if method == "" {
		method = http.MethodGet
	}
	targetPath, err := record.ResolveOperationPath(path, op, isCollection)
	if err != nil {
		fmt.Fprintf(out, "  %s: %s (path error: %v)\n", label, method, err)
		return
	}
	fmt.Fprintf(out, "  %s: %s %s\n", label, method, targetPath)
	headers := record.HeadersFor(op, path, isCollection)
	if len(headers) > 0 {
		lines := resource.HeaderListFromMap(headers)
		for _, line := range lines {
			fmt.Fprintf(out, "    header: %s\n", line)
		}
	}
	if query := record.QueryFor(op); len(query) > 0 {
		fmt.Fprintf(out, "    query: %s\n", formatQuery(query))
	}
	if op.URL != nil && len(op.URL.QueryStrings) > 0 {
		fmt.Fprintf(out, "    query strings: %s\n", strings.Join(op.URL.QueryStrings, ", "))
	}
}

func formatQuery(query map[string][]string) string {
	var parts []string
	for key, values := range query {
		if len(values) == 0 {
			parts = append(parts, fmt.Sprintf("%s=", key))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, strings.Join(values, ",")))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func describeOpenAPI(out io.Writer, spec *openapi.Spec, record resource.ResourceRecord, path string, isCollection bool) {
	if spec == nil {
		fmt.Fprintln(out, "  OpenAPI spec is not configured.")
		return
	}
	readOp := record.ReadOperation(isCollection)
	remotePath := path
	if readOp != nil {
		if resolved, err := record.ResolveOperationPath(path, readOp, isCollection); err == nil && strings.TrimSpace(resolved) != "" {
			remotePath = resolved
		}
	}
	if remotePath == "" {
		remotePath = "/"
	}
	item := spec.MatchPath(remotePath)
	if item == nil {
		fmt.Fprintf(out, "  No OpenAPI path matches %s\n", remotePath)
		return
	}
	fmt.Fprintf(out, "  Template: %s\n", item.Template)
	refs := printOpenAPIMethods(out, item)
	fmt.Fprintln(out, "\nSchema:")
	printSchemaDetails(out, spec, refs)
}

func printOpenAPIMethods(out io.Writer, item *openapi.PathItem) []string {
	var refs []string
	for _, method := range []string{"get", "post", "put", "patch", "delete"} {
		if op := item.Operation(method); op != nil {
			fmt.Fprintf(out, "    %s:\n", strings.ToUpper(op.Method))
			refs = append(refs, printOpenAPIMethodDetails(out, op)...)
		}
	}
	return refs
}

func printOpenAPIMethodDetails(out io.Writer, op *openapi.Operation) []string {
	const indent = "      "
	printLabeledField(out, indent, "summary", op.Summary)
	printLabeledField(out, indent, "description", op.Description)
	printContentLine(out, indent, "requests", op.RequestContentTypes)
	printContentLine(out, indent, "responses", op.ResponseContentTypes)
	var refs []string
	if ref := printSchemaLine(out, indent, "request schema", op.RequestSchemaRef, op.RequestSchema); ref != "" {
		refs = append(refs, ref)
	}
	if ref := printSchemaLine(out, indent, "response schema", op.ResponseSchemaRef, op.ResponseSchema); ref != "" {
		refs = append(refs, ref)
	}
	return refs
}

func printContentLine(out io.Writer, indent, label string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(out, "%s%s:\n", indent, label)
		return
	}
	fmt.Fprintf(out, "%s%s: %s\n", indent, label, strings.Join(values, ", "))
}

func printSchemaLine(out io.Writer, indent, label, ref string, schema map[string]any) string {
	if ref != "" {
		fmt.Fprintf(out, "%s%s: %s\n", indent, label, schemaNameFromRef(ref))
		return ref
	}
	if schema != nil {
		summary := schemaSummary(schema)
		if summary == "" {
			summary = "(inline schema)"
		}
		fmt.Fprintf(out, "%s%s: %s\n", indent, label, summary)
	}
	return ""
}

func schemaNameFromRef(ref string) string {
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		name := strings.TrimPrefix(ref, prefix)
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	return strings.TrimSpace(ref)
}

func printSchemaDetails(out io.Writer, spec *openapi.Spec, refs []string) {
	refs = uniqueRefs(refs)
	if len(refs) == 0 {
		fmt.Fprintln(out, "  Schema is not defined for this path.")
		return
	}
	for _, ref := range refs {
		name := schemaNameFromRef(ref)
		if schema, ok := spec.SchemaFromRef(ref); ok && schema != nil {
			fmt.Fprintf(out, "  %s:\n", name)
			lines := schemaLines(schema)
			if len(lines) == 0 {
				fmt.Fprintln(out, "    Schema exists but no details are available.")
				continue
			}
			for _, line := range lines {
				fmt.Fprintf(out, "    %s\n", line)
			}
			continue
		}
		fmt.Fprintf(out, "  %s (schema definition unavailable)\n", name)
	}
}

func uniqueRefs(refs []string) []string {
	seen := make(map[string]struct{}, len(refs))
	var result []string
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		result = append(result, ref)
	}
	return result
}

func printLabeledField(out io.Writer, indent, label, value string) {
	text := strings.TrimSpace(value)
	if text == "" {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	fmt.Fprintf(out, "%s%s: %s\n", indent, label, strings.TrimSpace(lines[0]))
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Fprintf(out, "%s  %s\n", indent, line)
	}
}

func schemaLines(schema map[string]any) []string {
	if len(schema) == 0 {
		return nil
	}
	var lines []string
	collectSchemaLines(schema, "", &lines)
	return lines
}

func collectSchemaLines(schema map[string]any, indent string, lines *[]string) {
	if schema == nil {
		return
	}
	if ref, ok := schema["$ref"].(string); ok && strings.TrimSpace(ref) != "" {
		*lines = append(*lines, fmt.Sprintf("%s$ref: %s", indent, ref))
		return
	}

	if summary := schemaSummary(schema); summary != "" {
		*lines = append(*lines, fmt.Sprintf("%s%s", indent, summary))
	}
	if desc := schemaDescription(schema); desc != "" {
		*lines = append(*lines, fmt.Sprintf("%sdescription: %s", indent, desc))
	}
	if req := schemaRequired(schema); len(req) > 0 {
		*lines = append(*lines, fmt.Sprintf("%srequired: %s", indent, strings.Join(req, ", ")))
	}

	if props, ok := schema["properties"].(map[string]any); ok && len(props) > 0 {
		*lines = append(*lines, fmt.Sprintf("%sproperties:", indent))
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			*lines = append(*lines, fmt.Sprintf("%s  %s:", indent, key))
			if propSchema, ok := props[key].(map[string]any); ok && len(propSchema) > 0 {
				collectSchemaLines(propSchema, indent+"    ", lines)
			} else {
				*lines = append(*lines, fmt.Sprintf("%s    %s", indent, formatSchemaValue(props[key])))
			}
		}
	}

	if items, ok := schema["items"].(map[string]any); ok && len(items) > 0 {
		*lines = append(*lines, fmt.Sprintf("%sitems:", indent))
		collectSchemaLines(items, indent+"  ", lines)
	}
}

func schemaSummary(schema map[string]any) string {
	var parts []string
	if typ, ok := schema["type"].(string); ok && strings.TrimSpace(typ) != "" {
		parts = append(parts, fmt.Sprintf("type=%s", typ))
	}
	if format, ok := schema["format"].(string); ok && strings.TrimSpace(format) != "" {
		parts = append(parts, fmt.Sprintf("format=%s", format))
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		var tokens []string
		for _, entry := range enum {
			if str, ok := entry.(string); ok {
				tokens = append(tokens, str)
			} else {
				tokens = append(tokens, fmt.Sprintf("%v", entry))
			}
		}
		if len(tokens) > 0 {
			parts = append(parts, fmt.Sprintf("enum=%s", strings.Join(tokens, ",")))
		}
	}
	return strings.Join(parts, " ")
}

func schemaDescription(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	if desc, ok := schema["description"].(string); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

func schemaRequired(schema map[string]any) []string {
	raw, ok := schema["required"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	var result []string
	for _, entry := range raw {
		if str, ok := entry.(string); ok && strings.TrimSpace(str) != "" {
			result = append(result, str)
		}
	}
	sort.Strings(result)
	return result
}

func formatSchemaValue(value any) string {
	if value == nil {
		return "null"
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func openapiSpecFromProvider(provider interface{}) *openapi.Spec {
	type specProvider interface {
		OpenAPISpec() *openapi.Spec
	}
	if provider, ok := provider.(specProvider); ok {
		return provider.OpenAPISpec()
	}
	return nil
}

const openAPIFromContextValue = "__from_openapi_context__"

func newResourceAddCommand() *cobra.Command {
	var (
		path        string
		filePath    string
		fromPath    string
		fromOpenAPI string
		overrides   []string
		applyRemote bool
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "add <path> [file]",
		Short: "Add a resource definition to the resource repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageError(cmd, "expected <path> [file]")
			}
			path = strings.TrimSpace(path)
			filePath = strings.TrimSpace(filePath)
			fromPath = strings.TrimSpace(fromPath)
			fromOpenAPI = strings.TrimSpace(fromOpenAPI)
			if len(args) > 0 {
				argPath := strings.TrimSpace(args[0])
				if argPath != "" {
					if path != "" && path != argPath {
						return usageError(cmd, "path specified twice")
					}
					if path == "" {
						path = argPath
					}
				}
			}
			if len(args) > 1 {
				argFile := strings.TrimSpace(args[1])
				if argFile != "" {
					if fromPath != "" {
						return usageError(cmd, "cannot combine --from-path with a file argument")
					}
					if filePath != "" && filePath != argFile {
						return usageError(cmd, "file specified twice")
					}
					if filePath == "" {
						filePath = argFile
					}
				}
			}
			useOpenAPI := cmd.Flags().Changed("from-openapi")
			openAPISource := ""
			if useOpenAPI {
				if fromOpenAPI == openAPIFromContextValue || fromOpenAPI == "" {
					openAPISource = ""
				} else {
					openAPISource = fromOpenAPI
				}
			}
			if path == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if filePath != "" && fromPath != "" {
				return usageError(cmd, "--file and --from-path cannot be used together")
			}
			if useOpenAPI && (filePath != "" || fromPath != "") {
				return usageError(cmd, "--from-openapi cannot be combined with --file or --from-path")
			}
			if filePath == "" && fromPath == "" && !useOpenAPI {
				return usageError(cmd, "either --file, --from-path, or --from-openapi is required")
			}
			if fromPath != "" {
				if err := validateLogicalPath(cmd, fromPath); err != nil {
					return err
				}
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			var res resource.Resource
			if useOpenAPI {
				res, err = resourceFromOpenAPI(recon, path, openAPISource)
			} else if fromPath != "" {
				res, err = recon.GetLocalResource(fromPath)
				if err != nil {
					return err
				}
				res, err = dropResourceID(res)
				if err != nil {
					return err
				}
			} else {
				res, err = loadResourceFromFile(filePath)
			}
			if err != nil {
				return err
			}

			if len(overrides) > 0 {
				res, err = applyResourceOverrides(res, overrides)
				if err != nil {
					return err
				}
			}

			targetPath, err := resolveAddTargetPath(recon, path, res)
			if err != nil {
				return err
			}
			if !force {
				exists, err := localResourceExists(recon, targetPath)
				if err != nil {
					return err
				}
				if exists {
					if targetPath != path {
						return fmt.Errorf("resource %s already exists in the repository (resolved from %s); use --force to overwrite", targetPath, path)
					}
					return fmt.Errorf("resource %s already exists in the repository; use --force to overwrite", targetPath)
				}
			}

			if err := saveLocalResourceWithSecrets(recon, path, res, true); err != nil {
				return err
			}

			if targetPath != path {
				successf(cmd, "added resource %s to the resource repository (resolved from %s)", targetPath, path)
			} else {
				successf(cmd, "added resource %s to the resource repository", targetPath)
			}

			if applyRemote {
				if err := recon.SaveRemoteResource(targetPath, res); err != nil {
					return wrapRemoteErrorWithDetails(err, targetPath)
				}
				successf(cmd, "applied remote resource %s", targetPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to add")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a JSON or YAML resource payload file")
	cmd.Flags().StringVar(&fromPath, "from-path", "", "Resource path in the repository to copy")
	cmd.Flags().StringVar(&fromOpenAPI, "from-openapi", "", "Build the resource from an OpenAPI schema (optional spec path; defaults to context)")
	cmd.Flags().Lookup("from-openapi").NoOptDefVal = openAPIFromContextValue
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "Override resource fields (key=value list, JSON object, or JSON/YAML file)")
	cmd.Flags().BoolVar(&applyRemote, "apply", false, "Apply the resource to the remote server after saving")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite the resource in the repository if it already exists")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func loadResourceFromFile(path string) (resource.Resource, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return resource.Resource{}, errors.New("resource file path is required")
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		return resource.Resource{}, err
	}

	doc, err := parseResourceDocument(data, filepath.Dir(trimmed))
	if err != nil {
		return resource.Resource{}, err
	}

	if format, ok := resourceFileFormat(trimmed); ok {
		return decodeResolvedResource(doc, format)
	}

	if res, err := decodeResolvedResource(doc, "json"); err == nil {
		return res, nil
	}
	if res, err := decodeResolvedResource(doc, "yaml"); err == nil {
		return res, nil
	}

	return resource.Resource{}, fmt.Errorf("resource file %q must be valid JSON or YAML", trimmed)
}

func decodeResourceData(data []byte, format string) (resource.Resource, error) {
	switch format {
	case "yaml":
		return resource.NewResourceFromYAML(data)
	default:
		return resource.NewResourceFromJSON(data)
	}
}

func resourceFileFormat(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "yaml", true
	case ".json":
		return "json", true
	default:
		return "", false
	}
}

func parseResourceDocument(data []byte, baseDir string) (any, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized, err := normalizeYAMLStructure(raw)
	if err != nil {
		return nil, err
	}
	return resolveResourceIncludes(normalized, baseDir)
}

func decodeResolvedResource(doc any, format string) (resource.Resource, error) {
	switch format {
	case "yaml":
		b, err := yaml.Marshal(doc)
		if err != nil {
			return resource.Resource{}, err
		}
		return resource.NewResourceFromYAML(b)
	default:
		b, err := json.Marshal(doc)
		if err != nil {
			return resource.Resource{}, err
		}
		return resource.NewResourceFromJSON(b)
	}
}

func resolveResourceIncludes(value any, baseDir string) (any, error) {
	switch t := value.(type) {
	case map[string]any:
		for key, item := range t {
			resolved, err := resolveResourceIncludes(item, baseDir)
			if err != nil {
				return nil, err
			}
			t[key] = resolved
		}
		return t, nil
	case []any:
		for idx, item := range t {
			resolved, err := resolveResourceIncludes(item, baseDir)
			if err != nil {
				return nil, err
			}
			t[idx] = resolved
		}
		return t, nil
	case string:
		includePath, ok := parseIncludeDirective(t)
		if !ok {
			return t, nil
		}
		return loadIncludedValue(baseDir, includePath)
	default:
		return t, nil
	}
}

func loadIncludedValue(baseDir, includePath string) (any, error) {
	resolved := includePath
	if !filepath.IsAbs(includePath) {
		resolved = filepath.Join(baseDir, includePath)
	}
	resolved = filepath.Clean(resolved)

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}

	if _, ok := resourceFileFormat(resolved); ok {
		return parseResourceDocument(data, filepath.Dir(resolved))
	}

	return string(data), nil
}

func normalizeYAMLStructure(value any) (any, error) {
	switch t := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[key] = norm
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			ks, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("yaml key %v is not a string", key)
			}
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[ks] = norm
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for idx, val := range t {
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[idx] = norm
		}
		return out, nil
	default:
		return t, nil
	}
}

func parseIncludeDirective(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 4 || !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
	const includeKeyword = "include"
	if !strings.HasPrefix(inner, includeKeyword) {
		return "", false
	}
	path := strings.TrimSpace(inner[len(includeKeyword):])
	if path == "" {
		return "", false
	}
	return path, true
}

func resolveAddTargetPath(recon *reconciler.DefaultReconciler, path string, res resource.Resource) (string, error) {
	if recon == nil || recon.ResourceRecordProvider == nil {
		return "", errors.New("resource record provider is not configured")
	}

	record, err := recon.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}

	payload := record.ReadPayload()
	processed, err := record.ApplyPayload(res, payload)
	if err != nil {
		return "", err
	}

	return record.AliasPath(processed), nil
}

func localResourceExists(recon *reconciler.DefaultReconciler, path string) (bool, error) {
	if recon == nil {
		return false, errors.New("reconciler is not configured")
	}
	_, err := recon.GetLocalResource(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func applyResourceOverrides(res resource.Resource, overrides []string) (resource.Resource, error) {
	obj, err := ensureResourceObject(res)
	if err != nil {
		return resource.Resource{}, err
	}

	for _, raw := range overrides {
		override, err := parseOverride(raw)
		if err != nil {
			return resource.Resource{}, err
		}
		mergeOverride(obj, override)
	}

	return resource.NewResource(obj)
}

func ensureResourceObject(res resource.Resource) (map[string]any, error) {
	if res.V == nil {
		return map[string]any{}, nil
	}
	obj, ok := res.AsObject()
	if !ok {
		return nil, errors.New("overrides require an object resource")
	}
	return obj, nil
}

func parseOverride(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("override value is required")
	}

	if json.Valid([]byte(trimmed)) {
		res, err := resource.NewResourceFromJSON([]byte(trimmed))
		if err != nil {
			return nil, err
		}
		obj, ok := res.AsObject()
		if !ok {
			return nil, errors.New("override JSON must be an object")
		}
		return obj, nil
	}

	if strings.Contains(trimmed, "=") {
		pairs := splitCommaList(trimmed)
		if len(pairs) == 0 {
			return nil, errors.New("override must include at least one key=value pair")
		}
		override, err := parseOverridePairs(pairs)
		if err != nil {
			return nil, err
		}
		return override, nil
	}

	if override, ok, err := loadOverrideFile(trimmed); ok {
		return override, err
	}

	return nil, errors.New("override must be a JSON object, key=value list, or JSON/YAML file")
}

func loadOverrideFile(path string) (map[string]any, bool, error) {
	format, ok := resourceFileFormat(path)
	if !ok {
		return nil, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, fmt.Errorf("override file %q not found", path)
		}
		return nil, true, err
	}
	if info.IsDir() {
		return nil, true, fmt.Errorf("override file %q is a directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, err
	}

	res, err := decodeResourceData(data, format)
	if err != nil {
		return nil, true, err
	}
	obj, ok := res.AsObject()
	if !ok {
		return nil, true, fmt.Errorf("override file %q must contain a JSON or YAML object", path)
	}
	return obj, true, nil
}

func parseOverridePairs(pairs []string) (map[string]any, error) {
	override := make(map[string]any)
	for _, pair := range pairs {
		key, rawValue, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("override %q must use key=value syntax", pair)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("override key is required")
		}
		value, err := parseOverrideValue(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, err
		}
		if err := setOverridePath(override, key, value); err != nil {
			return nil, err
		}
	}
	return override, nil
}

func parseOverrideValue(raw string) (any, error) {
	if raw == "" {
		return "", nil
	}
	if !json.Valid([]byte(raw)) {
		return raw, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func setOverridePath(obj map[string]any, path string, value any) error {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return errors.New("override key is required")
	}
	current := obj
	for idx, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return fmt.Errorf("invalid override key %q", path)
		}
		if idx == len(segments)-1 {
			current[segment] = value
			return nil
		}
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	return nil
}

func mergeOverride(dst map[string]any, src map[string]any) {
	for key, value := range src {
		srcMap, ok := value.(map[string]any)
		if !ok {
			dst[key] = value
			continue
		}
		existing, ok := dst[key].(map[string]any)
		if !ok {
			dst[key] = srcMap
			continue
		}
		mergeOverride(existing, srcMap)
	}
}

func dropResourceID(res resource.Resource) (resource.Resource, error) {
	obj, ok := res.AsObject()
	if !ok {
		return res, nil
	}

	clone := make(map[string]any, len(obj))
	for key, value := range obj {
		clone[key] = value
	}

	if _, ok := clone["id"]; ok {
		delete(clone, "id")
		return resource.NewResource(clone)
	}
	return res, nil
}

func resourceFromOpenAPI(recon *reconciler.DefaultReconciler, logicalPath, source string) (resource.Resource, error) {
	if recon == nil {
		return resource.Resource{}, errors.New("reconciler is not configured")
	}

	spec, err := resolveOpenAPISpec(recon, source)
	if err != nil {
		return resource.Resource{}, err
	}
	return openapi.BuildResourceFromSpec(spec, logicalPath)
}

func newResourceListCommand() *cobra.Command {
	var (
		path       string
		listRepo   bool
		listRemote bool
	)

	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List resource paths from the resource repository or remote server",
		Long:  "List resource paths from the resource repository or remote server. When --remote is set without --path, DeclaREST enumerates collection paths from the resource repository to drive remote listing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveOptionalArg(cmd, path, args, "path")
			if err != nil {
				return err
			}

			repoChanged := cmd.Flags().Changed("repo")
			if listRemote && listRepo && !repoChanged {
				listRepo = false
			}
			if listRepo && listRemote {
				return usageError(cmd, "--repo and --remote cannot be used together")
			}
			if !listRepo && !listRemote {
				return usageError(cmd, "at least one of --repo or --remote must be true")
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			var paths []string
			if path != "" {
				if err := validateLogicalPath(cmd, path); err != nil {
					return err
				}
				if listRemote {
					paths, err = recon.ListRemoteResourcePaths(path)
				} else {
					paths, err = recon.RepositoryPathsInCollection(path)
				}
			} else {
				if listRemote {
					paths, err = recon.ListRemoteResourcePathsFromLocal()
				} else {
					paths = recon.RepositoryResourcePaths()
				}
			}
			if err != nil {
				return err
			}

			if len(paths) == 0 {
				return nil
			}

			for _, path := range paths {
				infof(cmd, "%s", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Collection path to list (optional)")
	cmd.Flags().BoolVar(&listRepo, "repo", true, "List resources from the resource repository (default)")
	cmd.Flags().BoolVar(&listRemote, "remote", false, "List resources from the remote server (uses resource repository collection metadata when --path is omitted)")

	registerResourcePathCompletion(cmd, resourceListPathStrategy)

	return cmd
}

func newResourceCreateCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths = recon.RepositoryResourcePaths()
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.CreateRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
				}

				if debugEnabled(debugGroupResource) {
					successf(cmd, "created remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "created remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to create")
	cmd.Flags().BoolVar(&all, "all", false, "Create all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After creating, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	return cmd
}

func newResourceUpdateCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "update <path>",
		Short: "Update the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths = recon.RepositoryResourcePaths()
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.UpdateRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
				}
				if debugEnabled(debugGroupResource) {
					successf(cmd, "updated remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "updated remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to update")
	cmd.Flags().BoolVar(&all, "all", false, "Update all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After updating, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newResourceApplyCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "apply <path>",
		Short: "Create or update the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths = recon.RepositoryResourcePaths()
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.SaveRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
					if res, err := recon.GetLocalResource(target); err == nil {
						data = res
					}
				}

				if debugEnabled(debugGroupResource) {
					successf(cmd, "applied remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "applied remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to apply")
	cmd.Flags().BoolVar(&all, "all", false, "Apply all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After applying, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newResourceDeleteCommand() *cobra.Command {
	var (
		path         string
		all          bool
		repo         bool
		remote       bool
		resourceList bool
		allItems     bool
		yes          bool
	)

	cmd := &cobra.Command{
		Use:   "delete <path>",
		Short: "Delete resources from the resource repository, remote resources, or both",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			isCollection := !all && resource.IsCollectionPath(targetPath)
			resourceListChanged := cmd.Flags().Changed("resource-list")

			if all {
				if resourceListChanged || allItems {
					return usageError(cmd, "--resource-list and --all-items require --path")
				}
			} else {
				if resourceListChanged && !isCollection {
					return usageError(cmd, "--resource-list requires a collection path")
				}
				if allItems && !isCollection {
					return usageError(cmd, "--all-items requires a collection path")
				}
				if (resourceListChanged || allItems) && !repo {
					return usageError(cmd, "--resource-list and --all-items require --repo")
				}
				if repo && isCollection && !resourceListChanged {
					resourceList = true
				}
				if repo && isCollection && !resourceList && !allItems && !remote {
					return usageError(cmd, "no delete targets specified for collection path")
				}
			}

			if err := ensureDeleteTargets(cmd, remote, repo); err != nil {
				return err
			}

			confirmMessage := resourceDeleteConfirmationMessage(targetPath, all, isCollection, repo, remote, resourceList, allItems)
			if err := confirmAction(cmd, yes, confirmMessage); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths = recon.RepositoryResourcePaths()
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				deletedLocal := false
				deletedRemote := false

				if remote {
					if err := recon.DeleteRemoteResource(target); err != nil {
						return wrapRemoteErrorWithDetails(err, target)
					}
					deletedRemote = true
				}

				if repo {
					if !isCollection || resourceList {
						if err := recon.DeleteLocalResource(target); err != nil {
							return err
						}
						deletedLocal = true
					}
				}

				switch {
				case deletedLocal && deletedRemote:
					successf(cmd, "deleted resource %s from the resource repository and remote resource", target)
				case deletedRemote:
					successf(cmd, "deleted remote resource %s", target)
				case deletedLocal:
					successf(cmd, "deleted resource %s from the resource repository", target)
				}
			}

			if allItems {
				itemPaths, err := recon.RepositoryPathsInCollection(targetPath)
				if err != nil {
					return err
				}
				base := strings.TrimRight(resource.NormalizePath(targetPath), "/")
				for _, item := range itemPaths {
					if item == base {
						continue
					}
					if err := recon.DeleteLocalResource(item); err != nil {
						return err
					}
					successf(cmd, "deleted resource %s from the resource repository", item)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to delete")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all resources from the resource repository")
	cmd.Flags().BoolVar(&repo, "repo", true, "Delete from the resource repository (default unless --remote is set)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Delete remote resources")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&resourceList, "resource-list", false, "When used with --repo on a collection path, delete the collection list entry from the resource repository")
	cmd.Flags().BoolVar(&allItems, "all-items", false, "When used with --repo on a collection path, delete all saved collection items from the resource repository")

	registerResourcePathCompletion(cmd, resourceDeletePathStrategy)

	return cmd
}

func resourceDeleteConfirmationMessage(targetPath string, all, isCollection, repo, remote, resourceList, allItems bool) string {
	target := "resource"
	switch {
	case all:
		target = "all resources"
	case isCollection && resourceList && allItems:
		target = fmt.Sprintf("collection %s and all items under it", targetPath)
	case isCollection && allItems:
		target = fmt.Sprintf("all items under collection %s", targetPath)
	case isCollection && resourceList:
		target = fmt.Sprintf("collection entry %s", targetPath)
	case isCollection:
		target = fmt.Sprintf("collection %s", targetPath)
	default:
		target = fmt.Sprintf("resource %s", targetPath)
	}
	return fmt.Sprintf("Delete %s. %s Continue?", target, impactSummary(repo, remote))
}

func ensureDeleteTargets(cmd *cobra.Command, remote, repo bool) error {
	if !remote && !repo {
		return usageError(cmd, "at least one of --remote or --repo must be true")
	}
	return nil
}

func resolvePathOrAll(cmd *cobra.Command, path string, all bool, args []string) (string, error) {
	trimmed, err := resolveOptionalArg(cmd, path, args, "path")
	if err != nil {
		return "", err
	}
	if all {
		if strings.TrimSpace(trimmed) != "" {
			return "", usageError(cmd, "--all cannot be used with --path")
		}
		return "", nil
	}
	if strings.TrimSpace(trimmed) == "" {
		return "", usageError(cmd, "path is required unless --all is set")
	}
	if err := validateLogicalPath(cmd, trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func warnUnmappedSecrets(cmd *cobra.Command, path string, res resource.Resource, secretPaths []string) {
	unmapped := secrets.FindUnmappedSecretPaths(res, secretPaths, resource.IsCollectionPath(path))
	if len(unmapped) == 0 {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Warning: potential secrets in %s are not mapped to resourceInfo.secretInAttributes:\n", path)
	for _, attr := range unmapped {
		fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", attr)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Run `declarest secret check` to review or `declarest secret check --fix` to map and store them.")
}

func syncLocalResource(recon *reconciler.DefaultReconciler, path string) error {
	res, err := recon.GetRemoteResource(path)
	if err != nil {
		if managedserver.IsNotFoundError(err) {
			return nil
		}
		return wrapRemoteErrorWithDetails(err, path)
	}
	return saveLocalResourceWithSecrets(recon, path, res, true)
}

func ensureRepositoryOverwriteAllowed(recon *reconciler.DefaultReconciler, path string, force bool) error {
	if force || recon == nil || recon.ResourceRepositoryManager == nil {
		return nil
	}
	_, err := recon.GetLocalResource(path)
	if err == nil {
		return fmt.Errorf("resource %s already exists in the resource repository; use --force to override", path)
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func newResourceDiffCommand() *cobra.Command {
	var (
		path string
		fail bool
	)

	cmd := &cobra.Command{
		Use:   "diff <path>",
		Short: "Show the reconcile diff for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			patch, err := recon.DiffResource(path)
			if err != nil {
				return wrapRemoteErrorWithDetails(err, path)
			}

			if len(patch) == 0 {
				successf(cmd, "resource %s is in sync", path)
				return nil
			}

			if err := PrintPatchSummary(cmd, patch); err != nil {
				return err
			}
			if fail {
				return fmt.Errorf("resource %s is out of sync", path)
			}
			successf(cmd, "diff generated for %s", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to diff")
	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	cmd.Flags().BoolVar(&fail, "fail", false, "Exit with error if the resource is not in sync")
	return cmd
}

func printResourceJSON(cmd *cobra.Command, res resource.Resource) error {
	data, err := res.MarshalJSON()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')

	_, err = cmd.OutOrStdout().Write(buf.Bytes())
	return err
}

func PrintPatchSummary(cmd *cobra.Command, patch resource.ResourcePatch) error {
	for _, op := range patch {
		verb := strings.ToLower(strings.TrimSpace(op.Op))
		if verb == "" {
			verb = "change"
		}
		if strings.TrimSpace(op.Path) == "" {
			fmt.Fprintln(cmd.OutOrStdout(), verb)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", verb, op.Path)
	}
	return nil
}

func wrapRemoteErrorWithDetails(err error, path string) error {
	var httpErr *managedserver.HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.Status()
		if status == 0 {
			status = http.StatusInternalServerError
		}
		statusText := http.StatusText(status)
		if statusText == "" {
			statusText = "Unknown"
		}
		if managedserver.IsNotFoundError(err) {
			return fmt.Errorf("remote resource %s not found (HTTP %d %s)", path, status, statusText)
		}
		return fmt.Errorf("remote server returned %d %s for %s", status, statusText, path)
	}
	return err
}
