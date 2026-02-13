package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"

	ctx "github.com/crmarques/declarest/context"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"

	"github.com/spf13/cobra"
)

func newMetadataInferCommand() *cobra.Command {
	var (
		path      string
		specPath  string
		apply     bool
		idFrom    string
		aliasFrom string
		recursive bool
	)

	cmd := &cobra.Command{
		Use:   "infer <path>",
		Short: "Infer metadata attributes from an OpenAPI specification",
		Long: `Render suggested metadata for a collection or resource based on an OpenAPI spec.
Inference uses the configured spec by default and can be overridden with --spec.
Use --recursively to infer metadata for every collection in the OpenAPI descriptor that matches the supplied path.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			trimmedPath := strings.TrimSpace(path)
			if trimmedPath == "" {
				return usageError(cmd, "path is required")
			}
			metadataOnly := metadataForResourceOnly(cmd)
			logicalPath := resource.NormalizePath(trimmedPath)
			targetIsCollection := resource.IsCollectionPath(trimmedPath)
			normalizedPath, err := normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, normalizedPath); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			spec, err := resolveOpenAPISpec(recon, specPath)
			if err != nil {
				if errors.Is(err, errOpenAPISpecNotConfigured) {
					return errors.New("openapi spec is not configured; provide --spec or configure managed_server.http.openapi")
				}
				return err
			}

			if !targetIsCollection && !metadataOnly {
				if !openAPIPathEndsWithParameter(spec, logicalPath) {
					targetIsCollection = true
				}
			}

			overrides := metadata.InferenceOverrides{
				IDAttribute:    strings.TrimSpace(idFrom),
				AliasAttribute: strings.TrimSpace(aliasFrom),
			}

			if recursive {
				return runRecursiveMetadataInference(cmd, recon, spec, trimmedPath, apply, overrides)
			}

			result := metadata.InferResourceMetadata(spec, logicalPath, targetIsCollection, overrides)
			metadataTargetPath := normalizedPath
			if apply {
				metadataTargetPath = inferenceMetadataTargetPath(spec, logicalPath, normalizedPath, targetIsCollection, metadataOnly)
			}

			payload, err := json.MarshalIndent(metadataInferOutput{
				ResourceInfo:  &result.ResourceInfo,
				OperationInfo: result.OperationInfo,
				Reasons:       result.Reasons,
			}, "", "  ")
			if err != nil {
				return err
			}
			payload = append(payload, '\n')
			if _, err := cmd.OutOrStdout().Write(payload); err != nil {
				return err
			}

			if apply {
				if err := validateMetadataPath(cmd, metadataTargetPath); err != nil {
					return err
				}
				if result.ResourceInfo.IDFromAttribute == "" && result.ResourceInfo.AliasFromAttribute == "" && !operationInfoHasHeaders(result.OperationInfo) {
					infof(cmd, "nothing to apply for %s", metadataTargetPath)
					return nil
				}
				updatedAttrs := []string{}
				err = recon.UpdateLocalMetadata(metadataTargetPath, func(meta map[string]any) (bool, error) {
					updated, changed, err := setInferredResourceInfoAttributes(meta, result.ResourceInfo)
					if err != nil {
						return false, err
					}
					updatedAttrs = append(updatedAttrs, updated...)
					if result.OperationInfo != nil {
						headerUpdated, headerChanged, err := setInferredOperationHeaders(meta, result.OperationInfo)
						if err != nil {
							return false, err
						}
						updatedAttrs = append(updatedAttrs, headerUpdated...)
						changed = changed || headerChanged
					}
					return changed, nil
				})
				if err != nil {
					return err
				}
				if len(updatedAttrs) == 0 {
					infof(cmd, "metadata already contains the inferred attributes for %s", metadataTargetPath)
				} else {
					successf(cmd, "applied inferred metadata (%s) for %s", strings.Join(updatedAttrs, ", "), metadataTargetPath)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")
	cmd.Flags().StringVar(&specPath, "spec", "", "Path to an OpenAPI spec file (overrides configured spec)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the inferred metadata to the local file")
	cmd.Flags().BoolVar(&recursive, metadataInferRecursiveFlag, false, "Infer metadata for collections recursively via the OpenAPI spec")
	cmd.Flags().StringVar(&idFrom, "id-from", "", "Force a specific attribute to use as resource ID")
	cmd.Flags().StringVar(&aliasFrom, "alias-from", "", "Force a specific attribute to use as the alias")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func parseMetadataValue(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !json.Valid([]byte(trimmed)) {
		return raw, nil
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func metadataDefaultEditor() (string, error) {
	manager := &ctx.DefaultContextManager{}
	return manager.GetDefaultEditor()
}

func resolveEditorCommand(override, fallback string) ([]string, error) {
	editor := strings.TrimSpace(override)
	if editor == "" {
		editor = strings.TrimSpace(fallback)
	}
	if editor == "" {
		editor = "vi"
	}
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return nil, errors.New("editor command is empty")
	}
	return fields, nil
}

func runEditor(args []string) error {
	if len(args) == 0 {
		return errors.New("editor command is empty")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

func decodeMetadataEditFile(data []byte) (map[string]any, error) {
	cleaned := stripMetadataComments(data)
	if len(bytes.TrimSpace(cleaned)) == 0 {
		return map[string]any{}, nil
	}
	return decodeMetadataFile(cleaned)
}

func payloadToMetadataMap(payload any) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var meta map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&meta); err != nil {
		return nil, err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

func decodeMetadataFile(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("metadata file is empty")
	}

	var meta map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&meta); err != nil {
		return nil, err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

const metadataEditTemplateHeader = "=====\nThis template shows all available options and fills currently unset attributes with defaults. \nOnce you save this file, all attributes that still match the defaults together with comments will be removed from final file.\n=====\n"

var metadataEditComments = newMetadataEditComments()

func marshalMetadataEditPayloadWithComments(payload any) ([]byte, error) {
	metaMap, err := payloadToMetadataMap(payload)
	if err != nil {
		return nil, err
	}
	applyListCollectionTemplateDefaults(metaMap)

	var builder strings.Builder
	for _, line := range strings.Split(metadataEditTemplateHeader, "\n") {
		builder.WriteString("// ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteByte('\n')

	if err := encodeJSONWithComments(&builder, metaMap, "", metadataEditComments, 0); err != nil {
		return nil, err
	}
	builder.WriteByte('\n')

	return []byte(builder.String()), nil
}

func encodeJSONWithComments(w *strings.Builder, value any, path string, comments map[string]string, indent int) error {
	indentStr := strings.Repeat(" ", indent)
	switch val := value.(type) {
	case map[string]any:
		if len(val) == 0 {
			w.WriteString("{}")
			return nil
		}
		w.WriteString("{\n")
		keys := make([]string, 0, len(val))
		for key := range val {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for idx, key := range keys {
			childPath := joinJSONPath(path, key)
			entryIndent := strings.Repeat(" ", indent+2)
			if comment, ok := comments[childPath]; ok {
				w.WriteString(entryIndent)
				w.WriteString("// ")
				w.WriteString(comment)
				w.WriteByte('\n')
			}
			w.WriteString(entryIndent)
			w.WriteString("\"")
			w.WriteString(key)
			w.WriteString("\": ")
			if err := encodeJSONWithComments(w, val[key], childPath, comments, indent+2); err != nil {
				return err
			}
			if idx < len(keys)-1 {
				w.WriteString(",\n")
			} else {
				w.WriteString("\n")
			}
		}
		w.WriteString(indentStr)
		w.WriteString("}")
	case []any:
		if len(val) == 0 {
			w.WriteString("[]")
			return nil
		}
		w.WriteString("[\n")
		for idx, item := range val {
			childPath := fmt.Sprintf("%s[%d]", path, idx)
			entryIndent := strings.Repeat(" ", indent+2)
			w.WriteString(entryIndent)
			if err := encodeJSONWithComments(w, item, childPath, comments, indent+2); err != nil {
				return err
			}
			if idx < len(val)-1 {
				w.WriteString(",\n")
			} else {
				w.WriteString("\n")
			}
		}
		w.WriteString(indentStr)
		w.WriteString("]")
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return err
		}
		w.Write(data)
	}
	return nil
}

func joinJSONPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func stripMetadataComments(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	out := make([][]byte, 0, len(lines))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			out = append(out, line)
			continue
		}
		if isMetadataCommentLine(trimmed) {
			continue
		}
		out = append(out, line)
	}
	return bytes.Join(out, []byte("\n"))
}

func isMetadataCommentLine(line []byte) bool {
	return bytes.HasPrefix(line, []byte("//")) || bytes.HasPrefix(line, []byte("#"))
}

func newMetadataEditComments() map[string]string {
	comments := map[string]string{
		"resourceInfo":                                      "Resource-level overrides for ID, alias, collection path, and secrets.",
		"resourceInfo.collectionPath":                       "Absolute collection endpoint used for operations.",
		"resourceInfo.idFromAttribute":                      "Attribute to treat as the resource identifier instead of the path.",
		"resourceInfo.aliasFromAttribute":                   "Attribute to use as the alias or directory name.",
		"resourceInfo.secretInAttributes":                   "Dot paths treated as secrets; empty list clears inherited secrets.",
		"operationInfo":                                     "Operation-specific metadata overrides for CRUD and list actions.",
		"operationInfo.compareResources":                    "Comparison overrides used when diffing resources.",
		"operationInfo.compareResources.ignoreAttributes":   "Attributes ignored during comparisons.",
		"operationInfo.compareResources.suppressAttributes": "Attributes suppressed from the diff output.",
		"operationInfo.compareResources.filterAttributes":   "Attributes used to filter diffs.",
		"operationInfo.compareResources.jqExpression":       "jq expression applied before comparing resources.",
	}

	operations := map[string]string{
		"getResource":    "Metadata for fetching a single resource.",
		"createResource": "Metadata for creating a new resource.",
		"updateResource": "Metadata for updating an existing resource.",
		"deleteResource": "Metadata for deleting a resource.",
		"listCollection": "Metadata for listing collection items.",
	}
	for op, description := range operations {
		prefix := "operationInfo." + op
		comments[prefix] = description
		comments[prefix+".url.path"] = "Relative or templated path appended to collectionPath."
		comments[prefix+".url.queryStrings"] = "Query parameters (key=value or templated values)."
		comments[prefix+".httpMethod"] = "HTTP method used for this operation (auto-filled)."
		comments[prefix+".httpHeaders"] = "Headers sent with the request; values may reference templates."
		comments[prefix+".payload"] = "Payload transforms (suppress/filter/jq) before sending requests or trimming fetched items."
		comments[prefix+".payload.suppressAttributes"] = "Attributes removed from outgoing requests or fetched items."
		comments[prefix+".payload.filterAttributes"] = "Attributes whitelisted in outgoing requests or kept on fetched items."
		comments[prefix+".payload.jqExpression"] = "jq expression applied to outgoing payloads or fetched items."
		comments[prefix+".jqFilter"] = "jq expression run on list responses before alias/id matching; can filter/select items."
	}

	return comments
}

func applyListCollectionTemplateDefaults(meta map[string]any) {
	if meta == nil {
		return
	}
	opInfo := ensureMetadataMap(meta, "operationInfo")
	if opInfo == nil {
		return
	}

	list := ensureMetadataMap(opInfo, "listCollection")
	if list == nil {
		return
	}

	ensureStringField(list, "jqFilter", "")
	payload := ensureMetadataMap(list, "payload")
	ensureSliceField(payload, "filterAttributes")
	ensureSliceField(payload, "suppressAttributes")
	ensureStringField(payload, "jqExpression", "")
}

func ensureMetadataMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	if child, ok := parent[key]; ok {
		if cast, ok := child.(map[string]any); ok {
			return cast
		}
		return nil
	}
	child := map[string]any{}
	parent[key] = child
	return child
}

func ensureStringField(parent map[string]any, key, value string) {
	if parent == nil {
		return
	}
	if _, ok := parent[key]; !ok {
		parent[key] = value
	}
}

func ensureSliceField(parent map[string]any, key string) {
	if parent == nil {
		return
	}
	if _, ok := parent[key]; !ok {
		parent[key] = []any{}
	}
}

type secretListInput struct {
	values  []string
	replace bool
}

func formatMetadataOutput(meta resource.ResourceMetadata) (any, error) {
	output := metadataOutput{
		OperationInfo: meta.OperationInfo,
	}
	if meta.ResourceInfo == nil {
		values := []string{}
		output.ResourceInfo = &resourceInfoOutput{
			SecretInAttributes: &values,
		}
		return output, nil
	}

	outInfo := &resourceInfoOutput{
		IDFromAttribute:    meta.ResourceInfo.IDFromAttribute,
		AliasFromAttribute: meta.ResourceInfo.AliasFromAttribute,
		CollectionPath:     meta.ResourceInfo.CollectionPath,
	}
	values := meta.ResourceInfo.SecretInAttributes
	if values == nil {
		values = []string{}
	} else {
		values = append([]string{}, values...)
	}
	outInfo.SecretInAttributes = &values
	output.ResourceInfo = outInfo
	return output, nil
}

type metadataOutput struct {
	ResourceInfo  *resourceInfoOutput             `json:"resourceInfo,omitempty"`
	OperationInfo *resource.OperationInfoMetadata `json:"operationInfo,omitempty"`
}

type resourceInfoOutput struct {
	IDFromAttribute    string    `json:"idFromAttribute,omitempty"`
	AliasFromAttribute string    `json:"aliasFromAttribute,omitempty"`
	CollectionPath     string    `json:"collectionPath,omitempty"`
	SecretInAttributes *[]string `json:"secretInAttributes,omitempty"`
}

type metadataInferOutput struct {
	ResourceInfo  *resource.ResourceInfoMetadata  `json:"resourceInfo,omitempty"`
	OperationInfo *resource.OperationInfoMetadata `json:"operationInfo,omitempty"`
	Reasons       []string                        `json:"reasons,omitempty"`
}

type metadataInferRecursiveEntry struct {
	Path          string                          `json:"path"`
	ResourceInfo  *resource.ResourceInfoMetadata  `json:"resourceInfo,omitempty"`
	OperationInfo *resource.OperationInfoMetadata `json:"operationInfo,omitempty"`
	Reasons       []string                        `json:"reasons,omitempty"`
}

type metadataInferRecursiveOutput struct {
	Results []metadataInferRecursiveEntry `json:"results"`
}

func isSecretInAttributesAttribute(attribute string) bool {
	return strings.EqualFold(strings.TrimSpace(attribute), "resourceInfo.secretInAttributes")
}

func parseSecretInAttributesInput(raw string) (secretListInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return secretListInput{}, errors.New("secretInAttributes value is required")
	}

	if json.Valid([]byte(trimmed)) {
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var parsed any
		if err := dec.Decode(&parsed); err != nil {
			return secretListInput{}, err
		}
		switch value := parsed.(type) {
		case []any:
			if len(value) == 0 {
				return secretListInput{values: []string{}, replace: true}, nil
			}
			values, err := coerceStringSlice(value)
			if err != nil {
				return secretListInput{}, err
			}
			return secretListInput{values: values, replace: true}, nil
		case []string:
			return secretListInput{values: value, replace: true}, nil
		case string:
			if strings.Contains(value, ",") {
				values := splitCommaList(value)
				if len(values) == 0 {
					return secretListInput{}, errors.New("secretInAttributes must include at least one value")
				}
				return secretListInput{values: values, replace: true}, nil
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return secretListInput{}, errors.New("secretInAttributes must include at least one value")
			}
			return secretListInput{values: []string{value}, replace: false}, nil
		default:
			return secretListInput{}, errors.New("secretInAttributes must be a JSON array or comma-separated string")
		}
	}

	values := splitCommaList(trimmed)
	if len(values) == 0 {
		return secretListInput{}, errors.New("secretInAttributes must include at least one value")
	}
	replace := strings.Contains(trimmed, ",")
	return secretListInput{values: values, replace: replace}, nil
}

func setSecretInAttributes(meta map[string]any, attribute string, input secretListInput) (bool, error) {
	if input.replace {
		return metadata.SetMetadataAttribute(meta, attribute, input.values)
	}
	if len(input.values) == 0 {
		return metadata.SetMetadataAttribute(meta, attribute, []string{})
	}
	existing, ok := resource.GetAttrPath(meta, attribute)
	if ok && isArrayValue(existing) {
		return metadata.SetMetadataAttribute(meta, attribute, input.values[0])
	}
	return metadata.SetMetadataAttribute(meta, attribute, input.values)
}

func stripDefaultMetadata(current, defaults map[string]any) map[string]any {
	cleaned := map[string]any{}
	for key, value := range current {
		defaultValue, ok := defaults[key]
		if ok {
			if cleanedValue, keep := stripDefaultValue(value, defaultValue); keep {
				cleaned[key] = cleanedValue
			}
			continue
		}
		if shouldDropEmptyString(value) {
			continue
		}
		cleaned[key] = value
	}
	return cleaned
}

func stripDefaultValue(value, defaultValue any) (any, bool) {
	if shouldDropEmptyString(value) {
		return nil, false
	}
	valueMap, valueIsMap := value.(map[string]any)
	defaultMap, defaultIsMap := defaultValue.(map[string]any)
	if valueIsMap && defaultIsMap {
		nested := stripDefaultMetadata(valueMap, defaultMap)
		if len(nested) == 0 {
			return nil, false
		}
		return nested, true
	}
	if reflect.DeepEqual(value, defaultValue) {
		return nil, false
	}
	return value, true
}

func shouldDropEmptyString(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	return strings.TrimSpace(text) == ""
}

func splitCommaList(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func registerSecretAttributeValueCompletion(cmd *cobra.Command) {
	if cmd.Flag("value") == nil {
		return
	}
	_ = cmd.RegisterFlagCompletionFunc("value", secretAttributeValueCompletion)
}

func secretAttributeValueCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	attribute, err := cmd.Flags().GetString("attribute")
	if err != nil || !isSecretInAttributesAttribute(attribute) {
		return nil, cobra.ShellCompDirectiveDefault
	}
	metadataPath, isCollection := metadataCompletionTargetPath(cmd, args)
	if metadataPath == "" {
		return nil, cobra.ShellCompDirectiveDefault
	}

	recon, cleanup, err := resourcePathCompletionLoader(false)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil || recon == nil {
		return nil, cobra.ShellCompDirectiveDefault
	}

	candidates := secretAttributeCandidates(recon, metadataPath, isCollection)
	if len(candidates) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	base, prefix := splitCommaCompletionParts(toComplete)
	existing := make(map[string]struct{}, len(candidates))
	for _, entry := range splitCommaList(toComplete) {
		existing[entry] = struct{}{}
	}

	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := existing[candidate]; ok {
			continue
		}
		if prefix != "" && !strings.HasPrefix(candidate, prefix) {
			continue
		}
		matches = append(matches, base+candidate)
	}

	if len(matches) == 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	sort.Strings(matches)
	return matches, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func metadataCompletionTargetPath(cmd *cobra.Command, args []string) (string, bool) {
	rawPath := ""
	if len(args) > 0 {
		rawPath = strings.TrimSpace(args[0])
	}
	if flag := cmd.Flags().Lookup("path"); flag != nil {
		value := strings.TrimSpace(flag.Value.String())
		if value != "" {
			rawPath = value
		}
	}
	if rawPath == "" {
		return "", false
	}
	normalized, err := normalizeMetadataInputPath(cmd, rawPath)
	if err != nil {
		return "", false
	}
	return normalized, resource.IsCollectionPath(normalized)
}

func splitCommaCompletionParts(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	idx := strings.LastIndex(value, ",")
	if idx < 0 {
		return "", strings.TrimSpace(value)
	}
	baseEnd := idx + 1
	for baseEnd < len(value) && value[baseEnd] == ' ' {
		baseEnd++
	}
	return value[:baseEnd], strings.TrimSpace(value[baseEnd:])
}

func secretAttributeCandidates(recon reconciler.AppReconciler, metadataPath string, isCollection bool) []string {
	if recon == nil {
		return nil
	}
	candidateSet := make(map[string]struct{})

	addCandidate := func(value string) {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			candidateSet[trimmed] = struct{}{}
		}
	}

	if meta, err := recon.ResourceMetadata(metadataPath); err == nil && meta.ResourceInfo != nil {
		for _, attr := range meta.ResourceInfo.SecretInAttributes {
			addCandidate(attr)
		}
	}

	var paths []string
	if isCollection {
		if collectionPaths, err := recon.RepositoryPathsInCollection(metadataPath); err == nil {
			paths = append(paths, collectionPaths...)
		}
	} else {
		paths = append(paths, metadataPath)
	}

	for _, target := range paths {
		res, err := recon.GetLocalResource(target)
		if err != nil {
			continue
		}
		for _, attr := range secrets.FindUnmappedSecretPaths(res, nil, resource.IsCollectionPath(target)) {
			addCandidate(attr)
		}
	}

	if len(candidateSet) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(candidateSet))
	for value := range candidateSet {
		candidates = append(candidates, value)
	}
	sort.Strings(candidates)
	return candidates
}

func coerceStringSlice(values []any) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("secretInAttributes entries must be strings")
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, errors.New("secretInAttributes entries must be non-empty")
		}
		out = append(out, trimmed)
	}
	return out, nil
}

func isArrayValue(value any) bool {
	switch value.(type) {
	case []any, []string:
		return true
	default:
		return false
	}
}

func metadataForResourceOnly(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.Flags().Lookup(metadataResourceOnlyFlag) != nil {
		value, err := cmd.Flags().GetBool(metadataResourceOnlyFlag)
		if err == nil {
			return value
		}
	}
	if cmd.InheritedFlags().Lookup(metadataResourceOnlyFlag) != nil {
		value, err := cmd.InheritedFlags().GetBool(metadataResourceOnlyFlag)
		if err == nil {
			return value
		}
	}
	return false
}

func normalizeMetadataInputPath(cmd *cobra.Command, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", usageError(cmd, "path is required")
	}
	normalized := normalizeMetadataPath(trimmed, metadataForResourceOnly(cmd))
	return normalized, nil
}

func normalizeMetadataPath(path string, forResourceOnly bool) string {
	trimmed := strings.TrimSpace(path)
	isCollection := resource.IsCollectionPath(trimmed)
	normalized := resource.NormalizePath(trimmed)
	if isCollection || !forResourceOnly {
		if normalized != "/" {
			return normalized + "/"
		}
	}
	return normalized
}

func validateMetadataPath(cmd *cobra.Command, path string) error {
	if err := resource.ValidateMetadataPath(path); err != nil {
		return usageError(cmd, err.Error())
	}
	return nil
}

func runRecursiveMetadataInference(cmd *cobra.Command, recon reconciler.AppReconciler, spec *openapi.Spec, basePath string, apply bool, overrides metadata.InferenceOverrides) error {
	paths := openAPICollectionPaths(spec)
	filtered := filterCollectionPathsByPrefix(paths, basePath)

	entries := make([]metadataInferRecursiveEntry, 0, len(filtered))
	for _, candidate := range filtered {
		result := metadata.InferResourceMetadata(spec, candidate, true, overrides)
		info := result.ResourceInfo
		entries = append(entries, metadataInferRecursiveEntry{
			Path:          candidate,
			ResourceInfo:  &info,
			OperationInfo: result.OperationInfo,
			Reasons:       result.Reasons,
		})

		if !apply {
			continue
		}

		metadataPath := normalizeCollectionMetadataPath(candidate)
		if err := validateMetadataPath(cmd, metadataPath); err != nil {
			return err
		}

		if info.IDFromAttribute == "" && info.AliasFromAttribute == "" && !operationInfoHasHeaders(result.OperationInfo) {
			if !noStatusOutput {
				infof(cmd, "nothing to apply for %s", metadataPath)
			}
			continue
		}

		updatedAttrs := []string{}
		if err := recon.UpdateLocalMetadata(metadataPath, func(meta map[string]any) (bool, error) {
			updated, changed, err := setInferredResourceInfoAttributes(meta, info)
			if err != nil {
				return false, err
			}
			updatedAttrs = append(updatedAttrs, updated...)
			if result.OperationInfo != nil {
				headerUpdated, headerChanged, err := setInferredOperationHeaders(meta, result.OperationInfo)
				if err != nil {
					return false, err
				}
				updatedAttrs = append(updatedAttrs, headerUpdated...)
				changed = changed || headerChanged
			}
			return changed, nil
		}); err != nil {
			return err
		}

		if len(updatedAttrs) == 0 {
			if !noStatusOutput {
				infof(cmd, "metadata already contains the inferred attributes for %s", metadataPath)
			}
		} else if !noStatusOutput {
			successf(cmd, "applied inferred metadata (%s) for %s", strings.Join(updatedAttrs, ", "), metadataPath)
		}
	}

	payload, err := json.MarshalIndent(metadataInferRecursiveOutput{Results: entries}, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if _, err := cmd.OutOrStdout().Write(payload); err != nil {
		return err
	}

	return nil
}

func setInferredResourceInfoAttributes(meta map[string]any, info resource.ResourceInfoMetadata) ([]string, bool, error) {
	updated := []string{}
	changed := false
	if attr := strings.TrimSpace(info.IDFromAttribute); attr != "" {
		if ok, err := metadata.SetMetadataAttribute(meta, "resourceInfo.idFromAttribute", attr); err != nil {
			return nil, false, err
		} else if ok {
			changed = true
			updated = append(updated, "resourceInfo.idFromAttribute")
		}
	}
	if attr := strings.TrimSpace(info.AliasFromAttribute); attr != "" {
		if ok, err := metadata.SetMetadataAttribute(meta, "resourceInfo.aliasFromAttribute", attr); err != nil {
			return nil, false, err
		} else if ok {
			changed = true
			updated = append(updated, "resourceInfo.aliasFromAttribute")
		}
	}
	return updated, changed, nil
}

func operationInfoHasHeaders(info *resource.OperationInfoMetadata) bool {
	if info == nil {
		return false
	}
	ops := []*resource.OperationMetadata{
		info.ListCollection,
		info.CreateResource,
		info.GetResource,
		info.UpdateResource,
		info.DeleteResource,
	}
	for _, op := range ops {
		if op != nil && len(op.HTTPHeaders) > 0 {
			return true
		}
	}
	return false
}

func setInferredOperationHeaders(meta map[string]any, info *resource.OperationInfoMetadata) ([]string, bool, error) {
	updated := []string{}
	changed := false
	if info == nil {
		return updated, false, nil
	}
	candidates := []struct {
		op   *resource.OperationMetadata
		path string
	}{
		{info.ListCollection, "operationInfo.listCollection.httpHeaders"},
		{info.CreateResource, "operationInfo.createResource.httpHeaders"},
		{info.GetResource, "operationInfo.getResource.httpHeaders"},
		{info.UpdateResource, "operationInfo.updateResource.httpHeaders"},
		{info.DeleteResource, "operationInfo.deleteResource.httpHeaders"},
	}
	for _, candidate := range candidates {
		if candidate.op == nil || len(candidate.op.HTTPHeaders) == 0 {
			continue
		}
		if ok, err := metadata.SetMetadataAttribute(meta, candidate.path, candidate.op.HTTPHeaders); err != nil {
			return nil, false, err
		} else if ok {
			changed = true
			updated = append(updated, candidate.path)
		}
	}
	return updated, changed, nil
}

func openAPICollectionPaths(spec *openapi.Spec) []string {
	if spec == nil {
		return nil
	}
	seen := make(map[string]struct{})
	paths := make([]string, 0, len(spec.Paths))
	for _, item := range spec.Paths {
		if item == nil {
			continue
		}
		if item.Operation("post") == nil {
			continue
		}
		path := wildcardCollectionPath(item.Template)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func filterCollectionPathsByPrefix(paths []string, prefix string) []string {
	normalizedPrefix := resource.NormalizePath(strings.TrimSpace(prefix))
	if normalizedPrefix == "/" {
		return append([]string(nil), paths...)
	}
	filtered := make([]string, 0, len(paths))
	for _, candidate := range paths {
		if pathMatchesPrefix(candidate, normalizedPrefix) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func pathMatchesPrefix(candidate, prefix string) bool {
	candidate = resource.NormalizePath(candidate)
	prefix = resource.NormalizePath(prefix)
	candidateSegments := resource.SplitPathSegments(candidate)
	prefixSegments := resource.SplitPathSegments(prefix)
	if len(prefixSegments) == 0 {
		return true
	}
	if len(candidateSegments) < len(prefixSegments) {
		return false
	}
	for idx, prefixSeg := range prefixSegments {
		if !segmentsMatchWildcard(prefixSeg, candidateSegments[idx]) {
			return false
		}
	}
	return true
}

func openAPIPathEndsWithParameter(spec *openapi.Spec, path string) bool {
	if spec == nil {
		return false
	}
	item := spec.MatchPath(path)
	if item == nil || len(item.Segments) == 0 {
		return false
	}
	return isOpenAPIPathParameter(item.Segments[len(item.Segments)-1])
}

func segmentsMatchWildcard(a, b string) bool {
	if isWildcardSegment(a) || isWildcardSegment(b) {
		return true
	}
	return a == b
}

func isWildcardSegment(segment string) bool {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "_" {
		return true
	}
	return isOpenAPIPathParameter(trimmed)
}

func isOpenAPIPathParameter(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func wildcardCollectionPath(template string) string {
	normalized := resource.NormalizePath(template)
	segments := resource.SplitPathSegments(normalized)
	if len(segments) == 0 {
		return "/"
	}
	for idx, segment := range segments {
		if isOpenAPIPathParameter(segment) {
			segments[idx] = "_"
		}
	}
	return "/" + strings.Join(segments, "/")
}

func normalizeCollectionMetadataPath(path string) string {
	normalized := resource.NormalizePath(path)
	if normalized == "/" {
		return "/"
	}
	return normalized + "/"
}

func inferenceMetadataTargetPath(spec *openapi.Spec, logicalPath, normalizedPath string, targetIsCollection, metadataOnly bool) string {
	if spec == nil || !targetIsCollection || metadataOnly {
		return normalizedPath
	}
	if wildcard := wildcardCollectionPathForLogicalPath(spec, logicalPath); wildcard != "" {
		return normalizeCollectionMetadataPath(wildcard)
	}
	return normalizedPath
}

func wildcardCollectionPathForLogicalPath(spec *openapi.Spec, logicalPath string) string {
	if spec == nil {
		return ""
	}
	item := spec.MatchPath(resource.NormalizePath(logicalPath))
	if item == nil || item.Template == "" {
		return ""
	}
	return wildcardCollectionPath(item.Template)
}
