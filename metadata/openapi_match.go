package metadata

import (
	"path"
	"reflect"
	"sort"
	"strings"
)

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
			// Concrete selector segments can match templated OpenAPI variables
			// (for example /realms/publico-br against /realms/{realm}).
			score++
			continue
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
