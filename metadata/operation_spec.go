package metadata

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata/identitytemplate"
	"github.com/crmarques/declarest/resource"
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
	collectionPath, err := resolveEffectiveRemoteCollectionPath(metadata.RemoteCollectionPath, scopeCopy)
	if err != nil {
		return OperationSpec{}, err
	}
	scopeCopy["remoteCollectionPath"] = collectionPath

	spec := OperationSpec{
		Transforms: nil,
	}

	if metadata.Operations != nil {
		if operationSpec, found := metadata.Operations[string(operation)]; found {
			spec = MergeOperationSpec(spec, operationSpec)
		}
	}
	spec.Transforms = combineTransformSteps(metadata.Transforms, spec.Transforms)

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

func resolveEffectiveRemoteCollectionPath(rawCollectionPath string, scope map[string]any) (string, error) {
	candidate := strings.TrimSpace(rawCollectionPath)
	if candidate == "" {
		candidate = strings.TrimSpace(scopeString(scope["remoteCollectionPath"]))
	}
	if candidate == "" {
		return "", nil
	}

	rendered, err := renderTemplateString("remoteCollectionPath", candidate, scope)
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
			"relative metadata path requires remote collection path context",
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
		return "./{{/id}}"
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
	request InferenceRequest,
	openAPISpec any,
) (ResourceMetadata, error) {
	if request.Recursive {
		return ResourceMetadata{}, faults.NewTypedError(
			faults.ValidationError,
			"recursive metadata inference is not yet supported",
			nil,
		)
	}

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
		idFieldName, aliasFieldName := inferIdentityAttributes(
			target,
			openAPIIdentityAttribute,
			openAPIResourceAttributes,
		)
		if strings.TrimSpace(idFieldName) != "" {
			inferred.ID = identitytemplate.PointerTemplate(resource.JSONPointerForObjectKey(idFieldName))
		}
		if strings.TrimSpace(aliasFieldName) != "" {
			inferred.Alias = identitytemplate.PointerTemplate(resource.JSONPointerForObjectKey(aliasFieldName))
		}
		if shouldInferSecretAttribute(target) {
			inferred.SecretAttributes = []string{resource.JSONPointerForObjectKey("secret")}
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
		ID:                     inferred.ID,
		Alias:                  inferred.Alias,
		RequiredAttributes:     cloneStringSlice(inferred.RequiredAttributes),
		RemoteCollectionPath:   inferred.RemoteCollectionPath,
		Format:                 inferred.Format,
		Secret:                 cloneBoolPointer(inferred.Secret),
		SecretAttributes:       cloneStringSlice(inferred.SecretAttributes),
		ExternalizedAttributes: cloneExternalizedAttributes(inferred.ExternalizedAttributes),
		Operations:             cloneOperationMap(inferred.Operations),
		Transforms:             CloneTransformSteps(inferred.Transforms),
	}

	compact.Operations = removeDefaultOperationSpecs(compact.Operations, defaults.Operations)
	if len(compact.Operations) == 0 {
		compact.Operations = nil
	}
	if strings.TrimSpace(compact.RemoteCollectionPath) == strings.TrimSpace(defaults.RemoteCollectionPath) {
		compact.RemoteCollectionPath = ""
	}
	if strings.TrimSpace(compact.Format) == strings.TrimSpace(defaults.Format) {
		compact.Format = ""
	}

	return compact, nil
}

func renderOperationSpecTemplates(spec OperationSpec, scope map[string]any) (OperationSpec, error) {
	rendered := OperationSpec{
		Query:      maps.Clone(spec.Query),
		Headers:    maps.Clone(spec.Headers),
		Body:       resource.DeepCopyValue(spec.Body),
		Transforms: CloneTransformSteps(spec.Transforms),
		Validate:   cloneOperationValidationSpec(spec.Validate),
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
	for _, key := range slices.Sorted(maps.Keys(rendered.Query)) {
		value, renderErr := renderTemplateString("query."+key, rendered.Query[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Query[key] = value
	}

	for _, key := range slices.Sorted(maps.Keys(rendered.Headers)) {
		value, renderErr := renderTemplateString("headers."+key, rendered.Headers[key], scope)
		if renderErr != nil {
			return OperationSpec{}, renderErr
		}
		rendered.Headers[key] = value
	}

	for idx := range rendered.Transforms {
		if strings.TrimSpace(rendered.Transforms[idx].JQExpression) == "" {
			continue
		}
		rendered.Transforms[idx].JQExpression, err = renderTemplateString(
			"transforms["+strconv.Itoa(idx)+"].jqExpression",
			rendered.Transforms[idx].JQExpression,
			scope,
		)
		if err != nil {
			return OperationSpec{}, err
		}
	}

	return rendered, nil
}

// ValidateOperationSpecTemplates parses (but does not execute) all template
// strings in an OperationSpec. This catches malformed template syntax at
// metadata write time rather than deferring errors to render time.
func ValidateOperationSpecTemplates(label string, spec OperationSpec) error {
	validate := func(field string, raw string) error {
		if !strings.Contains(raw, "{{") {
			return nil
		}
		rewritten := rewriteMetadataTemplateSyntax(raw)
		_, err := template.New(field).Funcs(TemplateFuncMap(nil)).Parse(rewritten)
		if err != nil {
			return faults.NewTypedError(
				faults.ValidationError,
				fmt.Sprintf("invalid metadata template for %s %s", label, field),
				err,
			)
		}
		return nil
	}

	if err := validate("method", spec.Method); err != nil {
		return err
	}
	if err := validate("path", spec.Path); err != nil {
		return err
	}
	if err := validate("accept", spec.Accept); err != nil {
		return err
	}
	if err := validate("contentType", spec.ContentType); err != nil {
		return err
	}
	for key, value := range spec.Query {
		if err := validate("query."+key, value); err != nil {
			return err
		}
	}
	for key, value := range spec.Headers {
		if err := validate("headers."+key, value); err != nil {
			return err
		}
	}
	for idx, step := range spec.Transforms {
		if err := validate(fmt.Sprintf("transforms[%d].jqExpression", idx), step.JQExpression); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplateString(field string, raw string, scope map[string]any) (string, error) {
	if !strings.Contains(raw, "{{") {
		return raw, nil
	}

	rewritten := rewriteMetadataTemplateSyntax(raw)
	tmpl, err := template.New(field).Funcs(TemplateFuncMap(scope)).Option("missingkey=error").Parse(rewritten)
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
