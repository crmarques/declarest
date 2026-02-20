package metadata

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"reflect"
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

	spec := OperationSpec{
		Filter:   cloneStringSlice(metadata.Filter),
		Suppress: cloneStringSlice(metadata.Suppress),
		JQ:       metadata.JQ,
	}

	if metadata.Operations != nil {
		if operationSpec, found := metadata.Operations[string(operation)]; found {
			spec = MergeOperationSpec(spec, operationSpec)
		}
	}

	rendered, err := renderOperationSpecTemplates(spec, scope)
	if err != nil {
		return OperationSpec{}, err
	}

	if strings.TrimSpace(rendered.Path) == "" {
		return OperationSpec{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("metadata operation %q path is required", operation),
			nil,
		)
	}

	return rendered, nil
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
	openAPIMetadata, openAPIIdentityAttribute := inferMetadataFromOpenAPISpec(target, openAPISpec)
	inferred := MergeResourceMetadata(fallbackMetadata, openAPIMetadata)

	if target.Collection {
		idAttribute, aliasAttribute := inferIdentityAttributes(target, openAPIIdentityAttribute)
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
	openAPIDefaults, _ := inferMetadataFromOpenAPISpec(target, openAPISpec)
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
		Query:    cloneStringMap(spec.Query),
		Headers:  cloneStringMap(spec.Headers),
		Body:     spec.Body,
		Filter:   cloneStringSlice(spec.Filter),
		Suppress: cloneStringSlice(spec.Suppress),
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

	tmpl, err := template.New(field).Option("missingkey=error").Parse(raw)
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

func parseInferTarget(logicalPath string) (inferTarget, error) {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" {
		return inferTarget{}, faults.NewTypedError(faults.ValidationError, "logical path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(trimmed, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return inferTarget{}, faults.NewTypedError(faults.ValidationError, "logical path must be absolute", nil)
	}

	trailingCollectionMarker := strings.HasSuffix(normalizedInput, "/")
	rawSegments := strings.Split(normalizedInput, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return inferTarget{}, faults.NewTypedError(
				faults.ValidationError,
				"logical path must not contain traversal segments",
				nil,
			)
		}
		if hasWildcardPattern(segment) {
			if _, err := path.Match(segment, "sample"); err != nil {
				return inferTarget{}, faults.NewTypedError(
					faults.ValidationError,
					"logical path contains invalid wildcard expression",
					err,
				)
			}
		}
		segments = append(segments, segment)
	}

	collectionTarget := trailingCollectionMarker
	if len(segments) > 0 && segments[len(segments)-1] == "_" {
		collectionTarget = true
		segments = segments[:len(segments)-1]
	}
	for _, segment := range segments {
		if segment == "_" || hasWildcardPattern(segment) {
			collectionTarget = true
		}
	}

	selector := "/"
	if len(segments) > 0 {
		selector = "/" + strings.Join(segments, "/")
	}
	selector = path.Clean(selector)
	if !strings.HasPrefix(selector, "/") {
		return inferTarget{}, faults.NewTypedError(faults.ValidationError, "logical path must be absolute", nil)
	}
	if selector != "/" {
		selector = strings.TrimSuffix(selector, "/")
	}

	if !collectionTarget && selector == "/" {
		return inferTarget{}, faults.NewTypedError(
			faults.ValidationError,
			"resource metadata path must not target root",
			nil,
		)
	}

	return inferTarget{
		Selector:   selector,
		Segments:   splitPathSegments(selector),
		Collection: collectionTarget,
	}, nil
}

func inferFallbackMetadata(target inferTarget) ResourceMetadata {
	if !target.Collection {
		collectionPath := path.Dir(target.Selector)
		if collectionPath == "." || collectionPath == "" {
			collectionPath = "/"
		}

		return ResourceMetadata{
			CollectionPath: collectionPath,
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Method: "GET",
					Path:   target.Selector,
				},
				string(OperationCreate): {
					Method: "POST",
					Path:   target.Selector,
				},
				string(OperationUpdate): {
					Method: "PUT",
					Path:   target.Selector,
				},
				string(OperationDelete): {
					Method: "DELETE",
					Path:   target.Selector,
				},
				string(OperationList): {
					Method: "GET",
					Path:   collectionPath,
				},
				string(OperationCompare): {
					Method: "GET",
					Path:   target.Selector,
				},
			},
		}
	}

	idAttribute, _ := inferIdentityAttributes(target, "")
	collectionPath, resourcePath := inferCollectionAndResourceTemplatePaths(target, idAttribute)
	operations := make(map[string]OperationSpec)
	if collectionPath != "" {
		operations[string(OperationList)] = OperationSpec{Method: "GET", Path: collectionPath}
		operations[string(OperationCreate)] = OperationSpec{Method: "POST", Path: collectionPath}
	}
	if resourcePath != "" {
		operations[string(OperationGet)] = OperationSpec{Method: "GET", Path: resourcePath}
		operations[string(OperationUpdate)] = OperationSpec{Method: "PUT", Path: resourcePath}
		operations[string(OperationDelete)] = OperationSpec{Method: "DELETE", Path: resourcePath}
		operations[string(OperationCompare)] = OperationSpec{Method: "GET", Path: resourcePath}
	}

	return ResourceMetadata{
		CollectionPath: collectionPath,
		Operations:     operations,
	}
}

func inferMetadataFromOpenAPISpec(target inferTarget, openAPISpec any) (ResourceMetadata, string) {
	pathDefinitions := openAPIPathDefinitions(openAPISpec)
	if len(pathDefinitions) == 0 {
		return ResourceMetadata{}, ""
	}
	defaults := inferFallbackMetadata(target)

	var collectionCandidate openAPICandidate
	var resourceCandidate openAPICandidate

	if target.Collection {
		collectionCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments), pathDefinitions)
		resourceCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments)+1, pathDefinitions)
	} else {
		resourceCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments), pathDefinitions)
		collectionCandidate = selectOpenAPICandidate(splitPathSegments(path.Dir(target.Selector)), len(target.Segments)-1, pathDefinitions)
	}

	operations := make(map[string]OperationSpec)
	collectionPath := ""
	if collectionCandidate.path != "" {
		defaultCollectionPath := defaults.Operations[string(OperationList)].Path
		metadataCollectionPath := openAPIPathToMetadataTemplate(collectionCandidate.path, defaultCollectionPath)
		collectionPath = metadataCollectionPath
		if hasOpenAPIMethod(collectionCandidate.methods, "get") {
			operations[string(OperationList)] = OperationSpec{
				Method: "GET",
				Path:   metadataCollectionPath,
			}
		}
		if hasOpenAPIMethod(collectionCandidate.methods, "post") {
			operations[string(OperationCreate)] = OperationSpec{
				Method: "POST",
				Path:   metadataCollectionPath,
			}
		}
	}

	resourceIdentityAttribute := ""
	if resourceCandidate.path != "" {
		defaultResourcePath := defaults.Operations[string(OperationGet)].Path
		metadataResourcePath := openAPIPathToMetadataTemplate(resourceCandidate.path, defaultResourcePath)
		resourceIdentityAttribute, _ = lastOpenAPIVariable(resourceCandidate.segments)

		if hasOpenAPIMethod(resourceCandidate.methods, "get") {
			operations[string(OperationGet)] = OperationSpec{
				Method: "GET",
				Path:   metadataResourcePath,
			}
			operations[string(OperationCompare)] = OperationSpec{
				Method: "GET",
				Path:   metadataResourcePath,
			}
		}
		if hasOpenAPIMethod(resourceCandidate.methods, "put") {
			operations[string(OperationUpdate)] = OperationSpec{
				Method: "PUT",
				Path:   metadataResourcePath,
			}
		} else if hasOpenAPIMethod(resourceCandidate.methods, "patch") {
			operations[string(OperationUpdate)] = OperationSpec{
				Method: "PATCH",
				Path:   metadataResourcePath,
			}
		}
		if hasOpenAPIMethod(resourceCandidate.methods, "delete") {
			operations[string(OperationDelete)] = OperationSpec{
				Method: "DELETE",
				Path:   metadataResourcePath,
			}
		}
	}

	if len(operations) == 0 {
		return ResourceMetadata{}, resourceIdentityAttribute
	}
	return ResourceMetadata{
		CollectionPath: collectionPath,
		Operations:     operations,
	}, resourceIdentityAttribute
}

func promoteInferTargetFromOpenAPI(target inferTarget, openAPISpec any) inferTarget {
	if target.Collection {
		return target
	}

	pathDefinitions := openAPIPathDefinitions(openAPISpec)
	if len(pathDefinitions) == 0 {
		return target
	}

	exactMethods, found := pathDefinitions[target.Selector]
	if !found {
		return target
	}
	if !hasOpenAPIMethod(exactMethods, "get") && !hasOpenAPIMethod(exactMethods, "post") {
		return target
	}

	keys := make([]string, 0, len(pathDefinitions))
	for pathKey := range pathDefinitions {
		keys = append(keys, pathKey)
	}
	sort.Strings(keys)

	for _, pathKey := range keys {
		segments := splitPathSegments(pathKey)
		if len(segments) != len(target.Segments)+1 {
			continue
		}
		if !matchesExactSegmentPrefix(segments, target.Segments) {
			continue
		}

		lastSegment := segments[len(segments)-1]
		if _, isVariable := templateVariableName(lastSegment); !isVariable {
			continue
		}

		childMethods := pathDefinitions[pathKey]
		if hasOpenAPIMethod(childMethods, "get") ||
			hasOpenAPIMethod(childMethods, "put") ||
			hasOpenAPIMethod(childMethods, "patch") ||
			hasOpenAPIMethod(childMethods, "delete") {
			target.Collection = true
			return target
		}
	}

	return target
}

func matchesExactSegmentPrefix(candidate []string, prefix []string) bool {
	if len(prefix) > len(candidate) {
		return false
	}
	for idx := range prefix {
		if candidate[idx] != prefix[idx] {
			return false
		}
	}
	return true
}

func removeDefaultOperationSpecs(
	operations map[string]OperationSpec,
	defaults map[string]OperationSpec,
) map[string]OperationSpec {
	if len(operations) == 0 {
		return nil
	}

	filtered := make(map[string]OperationSpec, len(operations))
	keys := sortedOperationKeys(operations)
	for _, key := range keys {
		spec := operations[key]
		defaultSpec, hasDefault := defaults[key]
		if hasDefault && operationSpecsEquivalent(spec, defaultSpec) {
			continue
		}
		filtered[key] = spec
	}

	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func operationSpecsEquivalent(left OperationSpec, right OperationSpec) bool {
	normalizedLeft := normalizeOperationSpecForComparison(left)
	normalizedRight := normalizeOperationSpecForComparison(right)
	return reflect.DeepEqual(normalizedLeft, normalizedRight)
}

func normalizeOperationSpecForComparison(spec OperationSpec) OperationSpec {
	normalized := OperationSpec{
		Method:      strings.TrimSpace(spec.Method),
		Path:        strings.TrimSpace(spec.Path),
		Accept:      strings.TrimSpace(spec.Accept),
		ContentType: strings.TrimSpace(spec.ContentType),
		Body:        spec.Body,
		JQ:          strings.TrimSpace(spec.JQ),
	}

	if len(spec.Query) > 0 {
		normalized.Query = cloneStringMap(spec.Query)
	}
	if len(spec.Headers) > 0 {
		normalized.Headers = cloneStringMap(spec.Headers)
	}
	if len(spec.Filter) > 0 {
		normalized.Filter = cloneStringSlice(spec.Filter)
	}
	if len(spec.Suppress) > 0 {
		normalized.Suppress = cloneStringSlice(spec.Suppress)
	}

	return normalized
}

func openAPIPathDefinitions(openAPISpec any) map[string]map[string]struct{} {
	root, ok := asStringMap(openAPISpec)
	if !ok {
		return nil
	}

	pathsValue, ok := root["paths"]
	if !ok {
		return nil
	}
	pathsMap, ok := asStringMap(pathsValue)
	if !ok {
		return nil
	}

	result := make(map[string]map[string]struct{}, len(pathsMap))
	keys := make([]string, 0, len(pathsMap))
	for pathKey := range pathsMap {
		keys = append(keys, pathKey)
	}
	sort.Strings(keys)

	for _, pathKey := range keys {
		methods := openAPIPathMethods(pathsMap[pathKey])
		if len(methods) == 0 {
			continue
		}
		result[path.Clean("/"+strings.Trim(pathKey, "/"))] = methods
	}

	return result
}

func openAPIPathMethods(value any) map[string]struct{} {
	pathItem, ok := asStringMap(value)
	if !ok {
		return nil
	}

	methods := make(map[string]struct{})
	for method := range pathItem {
		switch strings.ToLower(strings.TrimSpace(method)) {
		case "get", "post", "put", "patch", "delete":
			methods[strings.ToLower(strings.TrimSpace(method))] = struct{}{}
		}
	}
	return methods
}

func selectOpenAPICandidate(
	selectorSegments []string,
	expectedSegments int,
	pathDefinitions map[string]map[string]struct{},
) openAPICandidate {
	best := openAPICandidate{score: -1}
	keys := make([]string, 0, len(pathDefinitions))
	for pathKey := range pathDefinitions {
		keys = append(keys, pathKey)
	}
	sort.Strings(keys)

	for _, pathKey := range keys {
		templateSegments := splitPathSegments(pathKey)
		if len(templateSegments) != expectedSegments {
			continue
		}

		match, score := matchOpenAPIPath(selectorSegments, templateSegments)
		if !match {
			continue
		}

		if score > best.score || (score == best.score && (best.path == "" || pathKey < best.path)) {
			best = openAPICandidate{
				path:     pathKey,
				segments: templateSegments,
				methods:  pathDefinitions[pathKey],
				score:    score,
			}
		}
	}

	if best.score < 0 {
		return openAPICandidate{}
	}
	return best
}

func hasOpenAPIPathMatch(
	selectorSegments []string,
	expectedSegments int,
	pathDefinitions map[string]map[string]struct{},
) bool {
	for pathKey := range pathDefinitions {
		templateSegments := splitPathSegments(pathKey)
		if len(templateSegments) != expectedSegments {
			continue
		}
		if matchOpenAPIPathForExistence(selectorSegments, templateSegments) {
			return true
		}
	}
	return false
}

func matchOpenAPIPathForExistence(selectorSegments []string, templateSegments []string) bool {
	if len(templateSegments) != len(selectorSegments) {
		return false
	}

	for idx := range selectorSegments {
		selectorSegment := selectorSegments[idx]
		templateSegment := templateSegments[idx]
		_, templateIsVariable := openAPIPathParameterName(templateSegment)

		if selectorSegment == "_" {
			continue
		}
		if hasWildcardPattern(selectorSegment) {
			if templateIsVariable {
				continue
			}
			matched, err := path.Match(selectorSegment, templateSegment)
			if err != nil || !matched {
				return false
			}
			continue
		}
		if templateIsVariable {
			continue
		}
		if selectorSegment != templateSegment {
			return false
		}
	}
	return true
}

func matchOpenAPIPath(selectorSegments []string, templateSegments []string) (bool, int) {
	selectorLength := len(selectorSegments)
	if selectorLength == 0 {
		return true, 0
	}
	if len(templateSegments) < selectorLength {
		return false, 0
	}

	score := 0
	for idx := 0; idx < selectorLength; idx++ {
		selectorSegment := selectorSegments[idx]
		templateSegment := templateSegments[idx]

		templateVariable, templateIsVariable := openAPIPathParameterName(templateSegment)
		if selectorSegment == "_" {
			if templateIsVariable {
				score += 2
			}
			continue
		}

		if hasWildcardPattern(selectorSegment) {
			if templateIsVariable {
				score++
				continue
			}
			matched, err := path.Match(selectorSegment, templateSegment)
			if err != nil || !matched {
				return false, 0
			}
			score++
			continue
		}

		if templateIsVariable {
			return false, 0
		}
		if selectorSegment != templateSegment {
			return false, 0
		}

		score += 3
		if templateVariable != "" {
			score++
		}
	}

	return true, score
}

func openAPIPathToMetadataTemplate(pathTemplate string, fallbackTemplate string) string {
	segments := splitPathSegments(pathTemplate)
	if len(segments) == 0 {
		return "/"
	}
	fallbackSegments := splitPathSegments(fallbackTemplate)

	converted := make([]string, 0, len(segments))
	for idx, segment := range segments {
		if variableName, isPathParameter := openAPIPathParameterName(segment); isPathParameter {
			if isTemplateIdentifier(variableName) {
				converted = append(converted, "{{."+variableName+"}}")
				continue
			}
			if idx < len(fallbackSegments) && isMetadataTemplatePlaceholderSegment(fallbackSegments[idx]) {
				converted = append(converted, fallbackSegments[idx])
				continue
			}
			converted = append(converted, "{{.id}}")
			continue
		}
		converted = append(converted, segment)
	}
	return "/" + strings.Join(converted, "/")
}

func hasOpenAPIMethod(methods map[string]struct{}, method string) bool {
	if len(methods) == 0 {
		return false
	}
	_, found := methods[strings.ToLower(strings.TrimSpace(method))]
	return found
}

func lastOpenAPIVariable(segments []string) (string, bool) {
	if len(segments) == 0 {
		return "", false
	}
	return templateVariableName(segments[len(segments)-1])
}

func templateVariableName(segment string) (string, bool) {
	parameterName, ok := openAPIPathParameterName(segment)
	if !ok || !isTemplateIdentifier(parameterName) {
		return "", false
	}
	return parameterName, true
}

func openAPIPathParameterName(segment string) (string, bool) {
	matches := openAPIPathParameterSegmentPattern.FindStringSubmatch(strings.TrimSpace(segment))
	if len(matches) != 2 {
		return "", false
	}
	parameterName := strings.TrimSpace(matches[1])
	if parameterName == "" {
		return "", false
	}
	return parameterName, true
}

func isTemplateIdentifier(value string) bool {
	return templateIdentifierPattern.MatchString(strings.TrimSpace(value))
}

func isMetadataTemplatePlaceholderSegment(segment string) bool {
	trimmed := strings.TrimSpace(segment)
	return strings.HasPrefix(trimmed, "{{.") && strings.HasSuffix(trimmed, "}}")
}

func inferCollectionAndResourceTemplatePaths(target inferTarget, resourceIdentity string) (string, string) {
	if !target.Collection {
		return "", ""
	}

	placeholderSuffix := strings.TrimSpace(resourceIdentity)
	if placeholderSuffix == "" {
		placeholderSuffix = "id"
	}

	collectionSegments := make([]string, 0, len(target.Segments))
	usedPlaceholderNames := make(map[string]int)
	for idx, segment := range target.Segments {
		if segment == "_" || hasWildcardPattern(segment) {
			placeholderName := inferPlaceholderName(target.Segments, idx, usedPlaceholderNames)
			collectionSegments = append(collectionSegments, "{{."+placeholderName+"}}")
			continue
		}
		collectionSegments = append(collectionSegments, segment)
	}

	collectionPath := "/"
	if len(collectionSegments) > 0 {
		collectionPath = "/" + strings.Join(collectionSegments, "/")
	}

	resourcePath := collectionPath
	if strings.TrimSpace(resourcePath) == "/" {
		resourcePath = "/{{." + placeholderSuffix + "}}"
	} else {
		resourcePath = resourcePath + "/{{." + placeholderSuffix + "}}"
	}

	return collectionPath, resourcePath
}

func inferPlaceholderName(
	segments []string,
	idx int,
	usedPlaceholderNames map[string]int,
) string {
	candidate := ""
	for previous := idx - 1; previous >= 0; previous-- {
		segment := strings.TrimSpace(segments[previous])
		if segment == "" || segment == "_" || hasWildcardPattern(segment) {
			continue
		}
		candidate = singularizeToken(segment)
		break
	}

	if candidate == "" {
		candidate = fmt.Sprintf("segment%d", idx+1)
	}

	counter := usedPlaceholderNames[candidate]
	usedPlaceholderNames[candidate] = counter + 1
	if counter > 0 {
		return fmt.Sprintf("%s%d", candidate, counter+1)
	}
	return candidate
}

func inferIdentityAttributes(target inferTarget, openAPIIdentityAttribute string) (string, string) {
	aliasAttribute := strings.TrimSpace(openAPIIdentityAttribute)
	if aliasAttribute == "" {
		collectionName := inferCollectionName(target)
		singularCollectionName := singularizeToken(collectionName)
		switch singularCollectionName {
		case "":
			aliasAttribute = "id"
		case "client":
			aliasAttribute = "clientId"
		default:
			aliasAttribute = singularCollectionName
		}
	}

	idAttribute := aliasAttribute
	if strings.HasSuffix(strings.ToLower(aliasAttribute), "id") && strings.ToLower(aliasAttribute) != "id" {
		idAttribute = "id"
	}

	return idAttribute, aliasAttribute
}

func inferCollectionName(target inferTarget) string {
	if len(target.Segments) == 0 {
		return ""
	}

	for idx := len(target.Segments) - 1; idx >= 0; idx-- {
		segment := strings.TrimSpace(target.Segments[idx])
		if segment == "" || segment == "_" || hasWildcardPattern(segment) {
			continue
		}
		return segment
	}
	return ""
}

func shouldInferSecretAttribute(target inferTarget) bool {
	collectionName := strings.ToLower(strings.TrimSpace(inferCollectionName(target)))
	if collectionName == "" {
		return false
	}

	return collectionName == "clients" ||
		strings.Contains(collectionName, "secret") ||
		strings.Contains(collectionName, "credential")
}

func singularizeToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}

	separatorNormalized := strings.ReplaceAll(strings.ReplaceAll(trimmed, "-", "_"), ".", "_")
	parts := strings.Split(separatorNormalized, "_")
	if len(parts) == 0 {
		return ""
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(last), "ies") && len(last) > 3 {
		last = last[:len(last)-3] + "y"
	} else if strings.HasSuffix(strings.ToLower(last), "s") && len(last) > 1 {
		last = last[:len(last)-1]
	}
	return last
}

func splitPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func hasWildcardPattern(segment string) bool {
	return strings.ContainsAny(segment, "*?[")
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		mapped := make(map[string]any, len(typed))
		for key, item := range typed {
			stringKey, ok := key.(string)
			if !ok {
				return nil, false
			}
			mapped[stringKey] = item
		}
		return mapped, true
	default:
		return nil, false
	}
}
