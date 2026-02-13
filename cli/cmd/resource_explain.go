package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"

	"github.com/spf13/cobra"
)

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

	record, err := recon.ResourceRecord(path)
	if err != nil {
		return err
	}

	spec := recon.OpenAPISpec()

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

	spec := recon.OpenAPISpec()
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

const openAPIFromContextValue = "__from_openapi_context__"
