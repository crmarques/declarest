package metadata

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
)

func ResolveOperationSpec(ctx context.Context, metadata ResourceMetadata, operation Operation, value any) (OperationSpec, error) {
	scope, err := buildTemplateScopeFromValue(value)
	if err != nil {
		return OperationSpec{}, err
	}
	return ResolveOperationSpecWithScope(ctx, metadata, operation, scope)
}

func ResolveOperationSpecWithScope(
	_ context.Context,
	metadata ResourceMetadata,
	operation Operation,
	scope map[string]any,
) (OperationSpec, error) {
	if !operation.IsValid() {
		return OperationSpec{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported metadata operation %q", operation),
			nil,
		)
	}

	scopeCopy := cloneScopeMap(scope)
	collectionPath, err := resolveEffectiveCollectionPath(metadata.CollectionPath, scopeCopy)
	if err != nil {
		return OperationSpec{}, err
	}
	scopeCopy["collectionPath"] = collectionPath

	spec := OperationSpec{
		Filter:                cloneStringSlice(metadata.Filter),
		Suppress:              cloneStringSlice(metadata.Suppress),
		JQ:                    metadata.JQ,
		PayloadTransformOrder: cloneStringSlice(metadata.PayloadTransformOrder),
	}

	if metadata.Operations != nil {
		if operationSpec, found := metadata.Operations[string(operation)]; found {
			spec = MergeOperationSpec(spec, operationSpec)
		}
	}

	if strings.TrimSpace(spec.Path) == "" {
		spec.Path = defaultOperationPathTemplate(operation)
	}

	rendered, err := renderOperationSpecTemplates(spec, scopeCopy)
	if err != nil {
		return OperationSpec{}, err
	}

	resolvedPath, err := resolveRenderedOperationPath(rendered.Path, collectionPath)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.Path = resolvedPath

	if strings.TrimSpace(rendered.Path) == "" {
		return OperationSpec{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("metadata operation %q path is required", operation),
			nil,
		)
	}

	return rendered, nil
}

func cloneScopeMap(scope map[string]any) map[string]any {
	if scope == nil {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(scope))
	for key, value := range scope {
		cloned[key] = value
	}
	return cloned
}

func resolveEffectiveCollectionPath(rawCollectionPath string, scope map[string]any) (string, error) {
	candidate := strings.TrimSpace(rawCollectionPath)
	if candidate == "" {
		candidate = strings.TrimSpace(scopeString(scope["collectionPath"]))
	}
	if candidate == "" {
		return "", nil
	}

	rendered, err := renderTemplateString("collectionPath", candidate, scope)
	if err != nil {
		return "", err
	}
	return normalizeRenderedOperationPath(rendered), nil
}

func resolveRenderedOperationPath(rawPath string, collectionPath string) (string, error) {
	trimmedPath := strings.TrimSpace(rawPath)
	if trimmedPath == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmedPath, "/") {
		return normalizeRenderedOperationPath(trimmedPath), nil
	}
	if !strings.HasPrefix(trimmedPath, ".") {
		return normalizeRenderedOperationPath(trimmedPath), nil
	}

	normalizedCollectionPath := strings.TrimSpace(collectionPath)
	if normalizedCollectionPath == "" {
		return "", faults.NewTypedError(
			faults.ValidationError,
			"relative metadata path requires collectionPath context",
			nil,
		)
	}

	joined := path.Join(normalizedCollectionPath, trimmedPath)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	return normalizeRenderedOperationPath(joined), nil
}

func normalizeRenderedOperationPath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return ""
	}

	normalized := path.Clean(trimmed)
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return normalized
}

func defaultOperationPathTemplate(operation Operation) string {
	switch operation {
	case OperationCreate, OperationList:
		return "."
	default:
		return "./{{.id}}"
	}
}

func scopeString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func InferFromOpenAPI(ctx context.Context, logicalPath string, request InferenceRequest) (ResourceMetadata, error) {
	return InferFromOpenAPISpec(ctx, logicalPath, request, nil)
}

func InferFromOpenAPISpec(
	_ context.Context,
	logicalPath string,
	_ InferenceRequest,
	openAPISpec any,
) (ResourceMetadata, error) {
	target, err := parseInferTarget(logicalPath)
	if err != nil {
		return ResourceMetadata{}, err
	}
	target = promoteInferTargetFromOpenAPI(target, openAPISpec)

	fallbackMetadata := inferFallbackMetadata(target)
	openAPIMetadata, openAPIIdentityAttribute, openAPIResourceAttributes := inferMetadataFromOpenAPISpec(
		target,
		openAPISpec,
	)
	inferred := MergeResourceMetadata(fallbackMetadata, openAPIMetadata)

	if target.Collection {
		idAttribute, aliasAttribute := inferIdentityAttributes(
			target,
			openAPIIdentityAttribute,
			openAPIResourceAttributes,
		)
		if strings.TrimSpace(idAttribute) != "" {
			inferred.IDFromAttribute = idAttribute
		}
		if strings.TrimSpace(aliasAttribute) != "" {
			inferred.AliasFromAttribute = aliasAttribute
		}
		if shouldInferSecretAttribute(target) {
			inferred.SecretsFromAttributes = []string{"secret"}
		}
	}

	return inferred, nil
}

func HasOpenAPIPath(logicalPath string, openAPISpec any) (bool, error) {
	target, err := parseInferTarget(logicalPath)
	if err != nil {
		return false, err
	}
	target = promoteInferTargetFromOpenAPI(target, openAPISpec)

	pathDefinitions := openAPIPathDefinitions(openAPISpec)
	if len(pathDefinitions) == 0 {
		return false, nil
	}

	if target.Collection {
		if hasOpenAPIPathMatch(target.Segments, len(target.Segments), pathDefinitions) {
			return true, nil
		}

		return hasOpenAPIPathMatch(target.Segments, len(target.Segments)+1, pathDefinitions), nil
	}

	return hasOpenAPIPathMatch(target.Segments, len(target.Segments), pathDefinitions), nil
}

func CompactInferredMetadataDefaults(logicalPath string, inferred ResourceMetadata, openAPISpec any) (ResourceMetadata, error) {
	target, err := parseInferTarget(logicalPath)
	if err != nil {
		return ResourceMetadata{}, err
	}
	target = promoteInferTargetFromOpenAPI(target, openAPISpec)

	defaults := inferFallbackMetadata(target)
	openAPIDefaults, _, _ := inferMetadataFromOpenAPISpec(target, openAPISpec)
	defaults = MergeResourceMetadata(defaults, openAPIDefaults)
	compact := ResourceMetadata{
		IDFromAttribute:       inferred.IDFromAttribute,
		AliasFromAttribute:    inferred.AliasFromAttribute,
		CollectionPath:        inferred.CollectionPath,
		SecretsFromAttributes: cloneStringSlice(inferred.SecretsFromAttributes),
		Operations:            cloneOperationMap(inferred.Operations),
		Filter:                cloneStringSlice(inferred.Filter),
		Suppress:              cloneStringSlice(inferred.Suppress),
		JQ:                    inferred.JQ,
	}

	compact.Operations = removeDefaultOperationSpecs(compact.Operations, defaults.Operations)
	if len(compact.Operations) == 0 {
		compact.Operations = nil
	}
	if strings.TrimSpace(compact.CollectionPath) == strings.TrimSpace(defaults.CollectionPath) {
		compact.CollectionPath = ""
	}

	return compact, nil
}

func renderOperationSpecTemplates(spec OperationSpec, scope map[string]any) (OperationSpec, error) {
	rendered := OperationSpec{
		Query:                 cloneStringMap(spec.Query),
		Headers:               cloneStringMap(spec.Headers),
		Body:                  spec.Body,
		Filter:                cloneStringSlice(spec.Filter),
		Suppress:              cloneStringSlice(spec.Suppress),
		Validate:              cloneOperationValidationSpec(spec.Validate),
		PayloadTransformOrder: cloneStringSlice(spec.PayloadTransformOrder),
	}

	var err error
	rendered.Method, err = renderTemplateString("method", spec.Method, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.Path, err = renderTemplateString("path", spec.Path, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.Accept, err = renderTemplateString("accept", spec.Accept, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.ContentType, err = renderTemplateString("contentType", spec.ContentType, scope)
	if err != nil {
		return OperationSpec{}, err
	}
	rendered.JQ, err = renderTemplateString("jq", spec.JQ, scope)
	if err != nil {
		return OperationSpec{}, err
	}

	for _, key := range sortedMapKeys(rendered.Query) {
		value, renderErr := renderTemplateString("query."+key, rendered.Query[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Query[key] = value
	}

	for _, key := range sortedMapKeys(rendered.Headers) {
		value, renderErr := renderTemplateString("headers."+key, rendered.Headers[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Headers[key] = value
	}

	return rendered, nil
}

func renderTemplateString(field string, raw string, scope map[string]any) (string, error) {
	if !strings.Contains(raw, "{{") {
		return raw, nil
	}

	tmpl, err := template.New(field).Funcs(TemplateFuncMap(scope)).Option("missingkey=error").Parse(raw)
	if err != nil {
		return "", faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("invalid metadata template for %s", field),
			err,
		)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, scope); err != nil {
		return "", faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("failed to render metadata template for %s", field),
			err,
		)
	}
	return buffer.String(), nil
}

func buildTemplateScopeFromValue(value any) (map[string]any, error) {
	scope := make(map[string]any)
	if payload, ok := value.(map[string]any); ok {
		for key, item := range payload {
			scope[key] = item
		}
		scope["payload"] = payload
	} else {
		scope["payload"] = value
	}
	scope["value"] = value

	return scope, nil
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var openAPIPathParameterSegmentPattern = regexp.MustCompile(`^\{([^{}]+)\}$`)
var templateIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type inferTarget struct {
	Selector   string
	Segments   []string
	Collection bool
}

type openAPICandidate struct {
	path     string
	segments []string
	methods  map[string]struct{}
	score    int
}
