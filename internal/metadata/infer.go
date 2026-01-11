package metadata

import (
	"fmt"
	"sort"
	"strings"

	"declarest/internal/openapi"
	"declarest/internal/resource"
)

// InferenceOverrides let callers force specific attribute names instead of
// relying purely on the OpenAPI heuristics.
type InferenceOverrides struct {
	IDAttribute    string
	AliasAttribute string
}

// InferenceResult captures the suggested metadata plus reasoning for humans.
type InferenceResult struct {
	ResourceInfo resource.ResourceInfoMetadata
	Reasons      []string
}

// InferResourceMetadata returns metadata suggestions derived from an OpenAPI
// spec for the given logical path. The logicalPath value should not include a
// trailing slash (normalize it with resource.NormalizePath before calling).
func InferResourceMetadata(spec *openapi.Spec, logicalPath string, isCollection bool, overrides InferenceOverrides) InferenceResult {
	logicalPath = resource.NormalizePath(logicalPath)
	result := InferenceResult{
		ResourceInfo: resource.ResourceInfoMetadata{
			CollectionPath: collectionPathFromLogicalPath(logicalPath, isCollection),
		},
	}
	if spec == nil {
		result.Reasons = append(result.Reasons, "OpenAPI spec is not available for inference")
		return result
	}

	matchPath := logicalPath
	if isCollection {
		matchPath = resource.NormalizePath(result.ResourceInfo.CollectionPath)
	}

	pathItem := spec.MatchPath(matchPath)
	if pathItem == nil {
		result.Reasons = append(result.Reasons, fmt.Sprintf("OpenAPI spec has no path matching %q", matchPath))
	}

	var schema map[string]any
	if isCollection {
		schema = openapi.CollectionRequestSchema(spec, matchPath)
	} else {
		schema = openapi.ResourceRequestSchema(spec, matchPath)
	}
	if schema == nil {
		result.Reasons = append(result.Reasons, fmt.Sprintf("no request schema is defined for %q in the OpenAPI spec", matchPath))
	}

	pathParams := extractPathParameters(pathItem)
	if isCollection {
		childParams := collectionResourcePathParameters(spec, matchPath)
		if len(childParams) > 0 {
			existing := make(map[string]struct{}, len(pathParams)+len(childParams))
			for _, name := range pathParams {
				existing[name] = struct{}{}
			}
			for _, name := range childParams {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				if _, ok := existing[name]; ok {
					continue
				}
				pathParams = append(pathParams, name)
				existing[name] = struct{}{}
			}
		}
	}

	idAttr, idReason := inferIDFromSchema(schema, pathParams, overrides)
	if idAttr != "" {
		result.ResourceInfo.IDFromAttribute = idAttr
	}
	if idReason != "" {
		result.Reasons = append(result.Reasons, idReason)
	}

	aliasAttr, aliasReason := inferAliasFromSchema(schema, pathParams, overrides)
	if aliasAttr != "" {
		result.ResourceInfo.AliasFromAttribute = aliasAttr
	}
	if aliasReason != "" {
		result.Reasons = append(result.Reasons, aliasReason)
	}

	return result
}

func collectionPathFromLogicalPath(logicalPath string, isCollection bool) string {
	trimmed := strings.Trim(logicalPath, " /")
	segments := resource.SplitPathSegments(trimmed)
	if !isCollection && len(segments) > 0 {
		segments = segments[:len(segments)-1]
	}
	return collectionPathFromSegmentsForInference(segments)
}

func collectionPathFromSegmentsForInference(segments []string) string {
	if len(segments) == 0 {
		return "/"
	}
	return resource.NormalizePath("/" + strings.Join(segments, "/"))
}

func collectionResourcePathParameters(spec *openapi.Spec, collectionPath string) []string {
	if spec == nil {
		return nil
	}
	normalized := resource.NormalizePath(collectionPath)
	baseSegments := resource.SplitPathSegments(normalized)
	type candidate struct {
		template string
		name     string
	}
	var candidates []candidate

	for _, item := range spec.Paths {
		if item == nil {
			continue
		}
		segments := item.Segments
		if len(segments) != len(baseSegments)+1 {
			continue
		}
		if !segmentsStartWith(segments, baseSegments) {
			continue
		}
		last := segments[len(segments)-1]
		if !isOpenAPIPathParameter(last) {
			continue
		}
		name := strings.TrimSpace(last[1 : len(last)-1])
		if name == "" {
			continue
		}
		candidates = append(candidates, candidate{template: item.Template, name: name})
	}

	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].template < candidates[j].template
	})

	seen := make(map[string]struct{}, len(candidates))
	params := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		if _, ok := seen[cand.name]; ok {
			continue
		}
		seen[cand.name] = struct{}{}
		params = append(params, cand.name)
	}
	return params
}

func segmentsStartWith(segments, prefix []string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(segments) < len(prefix) {
		return false
	}
	for idx, seg := range prefix {
		if segments[idx] != seg {
			return false
		}
	}
	return true
}

func isOpenAPIPathParameter(segment string) bool {
	segment = strings.TrimSpace(segment)
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func extractPathParameters(item *openapi.PathItem) []string {
	if item == nil || item.Template == "" {
		return nil
	}
	return parsePathTemplateParameters(item.Template)
}

func parsePathTemplateParameters(template string) []string {
	trimmed := strings.TrimSpace(template)
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return nil
	}

	segments := strings.Split(trimmed, "/")
	var params []string
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if len(segment) >= 3 && strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSpace(segment[1 : len(segment)-1])
			if name != "" {
				params = append(params, name)
			}
		}
	}
	return params
}

func inferIDFromSchema(schema map[string]any, pathParams []string, overrides InferenceOverrides) (string, string) {
	if overrides.IDAttribute != "" {
		return overrides.IDAttribute, fmt.Sprintf("idFromAttribute forced to %q via --id-from", overrides.IDAttribute)
	}

	idx := newPropertyIndex(schema)
	if idx != nil {
		for i := len(pathParams) - 1; i >= 0; i-- {
			if name, ok := idx.find(pathParams[i]); ok {
				return name, fmt.Sprintf("idFromAttribute inferred from schema property %q matching path parameter %q", name, pathParams[i])
			}
		}

		idCandidates := []string{"id", "uuid", "identifier"}
		if name, ok := findPropertyFromList(idx, idCandidates, true); ok {
			return name, fmt.Sprintf("idFromAttribute inferred from required schema property %q", name)
		}
		if name, ok := findPropertyFromList(idx, idCandidates, false); ok {
			return name, fmt.Sprintf("idFromAttribute inferred from schema property %q", name)
		}
		if name, ok := idx.findSuffix("id", true); ok {
			return name, fmt.Sprintf("idFromAttribute inferred from required schema property %q with an \"id\" suffix", name)
		}
		if name, ok := idx.findSuffix("id", false); ok {
			return name, fmt.Sprintf("idFromAttribute inferred from schema property %q with an \"id\" suffix", name)
		}
	}

	if len(pathParams) > 0 {
		last := pathParams[len(pathParams)-1]
		return last, fmt.Sprintf("idFromAttribute inferred from path parameter %q", last)
	}

	return "", ""
}

func inferAliasFromSchema(schema map[string]any, pathParams []string, overrides InferenceOverrides) (string, string) {
	if overrides.AliasAttribute != "" {
		return overrides.AliasAttribute, fmt.Sprintf("aliasFromAttribute forced to %q via --alias-from", overrides.AliasAttribute)
	}

	idx := newPropertyIndex(schema)
	aliasCandidates := []string{"name", "displayName", "title", "label", "slug", "alias", "clientId"}
	if name, ok := findPropertyFromList(idx, aliasCandidates, true); ok {
		return name, fmt.Sprintf("aliasFromAttribute inferred from required schema property %q", name)
	}
	if name, ok := findPropertyFromList(idx, aliasCandidates, false); ok {
		return name, fmt.Sprintf("aliasFromAttribute inferred from schema property %q", name)
	}

	if len(pathParams) > 0 {
		last := pathParams[len(pathParams)-1]
		return last, fmt.Sprintf("aliasFromAttribute defaulted to path parameter %q", last)
	}

	return "", ""
}

type propertyIndex struct {
	properties  map[string]any
	lowerNames  map[string]string
	required    map[string]struct{}
	sortedNames []string
}

func newPropertyIndex(schema map[string]any) *propertyIndex {
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return nil
	}
	idx := &propertyIndex{
		properties:  props,
		lowerNames:  make(map[string]string, len(props)),
		required:    make(map[string]struct{}),
		sortedNames: make([]string, 0, len(props)),
	}
	for name := range props {
		idx.sortedNames = append(idx.sortedNames, name)
		idx.lowerNames[strings.ToLower(name)] = name
	}
	sort.Strings(idx.sortedNames)

	if requiredList, ok := schema["required"].([]any); ok {
		for _, entry := range requiredList {
			if name, ok := entry.(string); ok && name != "" {
				idx.required[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
			}
		}
	}

	return idx
}

func (idx *propertyIndex) find(name string) (string, bool) {
	if idx == nil || strings.TrimSpace(name) == "" {
		return "", false
	}
	if _, ok := idx.properties[name]; ok {
		return name, true
	}
	if actual, ok := idx.lowerNames[strings.ToLower(name)]; ok {
		return actual, true
	}
	return "", false
}

func (idx *propertyIndex) isRequired(name string) bool {
	if idx == nil || strings.TrimSpace(name) == "" {
		return false
	}
	_, ok := idx.required[strings.ToLower(name)]
	return ok
}

func (idx *propertyIndex) findSuffix(suffix string, require bool) (string, bool) {
	if idx == nil || strings.TrimSpace(suffix) == "" {
		return "", false
	}
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	for _, name := range idx.sortedNames {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			if !require || idx.isRequired(name) {
				return name, true
			}
		}
	}
	return "", false
}

func findPropertyFromList(idx *propertyIndex, candidates []string, require bool) (string, bool) {
	if idx == nil {
		return "", false
	}
	for _, candidate := range candidates {
		if name, ok := idx.find(candidate); ok {
			if !require || idx.isRequired(name) {
				return name, true
			}
		}
	}
	return "", false
}
