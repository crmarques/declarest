package templatescope

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var jqResourcePathPattern = regexp.MustCompile(`resource\(\s*"((?:[^"\\]|\\.)*)"\s*\)`)

type ResourceScopeOptions struct {
	DerivedCollectionPath string
}

func BuildOperationScope(
	logicalPath string,
	collectionPath string,
	alias string,
	remoteID string,
	payload resource.Value,
) (map[string]any, error) {
	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return nil, err
	}

	scope := map[string]any{
		"logicalPath":           logicalPath,
		"logicalCollectionPath": collectionPath,
		"remoteCollectionPath":  collectionPath,
		"alias":                 alias,
		"remoteID":              remoteID,
		"payload":               normalizedPayload,
		"value":                 normalizedPayload,
	}

	if strings.TrimSpace(remoteID) != "" {
		scope["id"] = remoteID
	}

	if payloadMap, ok := normalizedPayload.(map[string]any); ok {
		for key, item := range payloadMap {
			scope[key] = item
		}
		scope["payload"] = payloadMap
		scope["value"] = payloadMap
	}

	return scope, nil
}

func BuildResourceScope(resource resource.Resource, md metadata.ResourceMetadata) (map[string]any, error) {
	return BuildResourceScopeWithOptions(resource, md, ResourceScopeOptions{})
}

func BuildResourceScopeWithOptions(
	resource resource.Resource,
	md metadata.ResourceMetadata,
	options ResourceScopeOptions,
) (map[string]any, error) {
	collectionPath := resource.CollectionPath
	if strings.TrimSpace(collectionPath) == "" {
		collectionPath = collectionPathForLogicalPath(resource.LogicalPath)
	}
	derivedCollectionPath := strings.TrimSpace(options.DerivedCollectionPath)
	if derivedCollectionPath == "" {
		derivedCollectionPath = collectionPath
	}

	scope, err := BuildOperationScope(
		resource.LogicalPath,
		collectionPath,
		resource.LocalAlias,
		resource.RemoteID,
		resource.Payload,
	)
	if err != nil {
		return nil, err
	}

	payloadMap, _ := scope["payload"].(map[string]any)
	for key, value := range DerivePathTemplateFields(resource.LogicalPath, md) {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if _, exists := scope[trimmedKey]; exists {
			continue
		}
		scope[trimmedKey] = trimmedValue
		if payloadMap != nil {
			payloadMap[trimmedKey] = trimmedValue
		}
	}
	for key, value := range deriveFieldsFromLogicalCollectionPath(derivedCollectionPath) {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if _, exists := scope[trimmedKey]; exists {
			continue
		}
		scope[trimmedKey] = trimmedValue
		if payloadMap != nil {
			payloadMap[trimmedKey] = trimmedValue
		}
	}

	return scope, nil
}

func DerivePathTemplateFields(logicalPath string, md metadata.ResourceMetadata) map[string]string {
	derived := map[string]string{}
	collectionTemplate := strings.TrimSpace(md.RemoteCollectionPath)
	if collectionTemplate == "" {
		collectionTemplate = collectionPathForLogicalPath(logicalPath)
	}
	mergeTemplateFields(derived, deriveTemplateFieldsFromPathTemplate(collectionTemplate, logicalPath))
	mergeTemplateFields(derived, deriveTemplateFieldsFromTransforms(md.Transforms, logicalPath))

	operationNames := make([]string, 0, len(md.Operations))
	for operationName := range md.Operations {
		operationNames = append(operationNames, operationName)
	}
	sort.Strings(operationNames)

	for _, operationName := range operationNames {
		spec := md.Operations[operationName]
		mergeTemplateFields(derived, deriveTemplateFieldsFromTransforms(spec.Transforms, logicalPath))

		templatePath := strings.TrimSpace(spec.Path)
		if templatePath == "" {
			continue
		}
		if !strings.HasPrefix(templatePath, "/") {
			templatePath = joinTemplatePaths(collectionTemplate, templatePath)
		}
		mergeTemplateFields(derived, deriveTemplateFieldsFromPathTemplate(templatePath, logicalPath))
	}

	return derived
}

func deriveTemplateFieldsFromTransforms(
	steps []metadata.TransformStep,
	logicalPath string,
) map[string]string {
	if len(steps) == 0 {
		return nil
	}

	derived := map[string]string{}
	for _, step := range steps {
		mergeTemplateFields(derived, deriveTemplateFieldsFromJQExpression(step.JQExpression, logicalPath))
	}
	if len(derived) == 0 {
		return nil
	}
	return derived
}

func deriveTemplateFieldsFromJQExpression(jqExpression string, logicalPath string) map[string]string {
	trimmedExpression := strings.TrimSpace(jqExpression)
	if trimmedExpression == "" || !strings.Contains(trimmedExpression, "{{") || !strings.Contains(trimmedExpression, "resource(") {
		return nil
	}

	matches := jqResourcePathPattern.FindAllStringSubmatch(trimmedExpression, -1)
	if len(matches) == 0 {
		return nil
	}

	derived := map[string]string{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		pathTemplate, unquoteErr := strconv.Unquote(`"` + match[1] + `"`)
		if unquoteErr != nil {
			continue
		}
		mergeTemplateFields(
			derived,
			deriveTemplateFieldsFromPathTemplate(strings.TrimSpace(pathTemplate), logicalPath),
		)
	}

	if len(derived) == 0 {
		return nil
	}
	return derived
}

func deriveTemplateFieldsFromPathTemplate(pathTemplate string, logicalPath string) map[string]string {
	if !strings.Contains(pathTemplate, "{{") {
		return nil
	}

	templateSegments := splitPathSegments(pathTemplate)
	logicalSegments := splitPathSegments(logicalPath)
	if len(templateSegments) == 0 || len(logicalSegments) == 0 {
		return nil
	}

	fields := make(map[string]string)
	segmentLimit := len(templateSegments)
	if len(logicalSegments) < segmentLimit {
		segmentLimit = len(logicalSegments)
	}

	for idx := 0; idx < segmentLimit; idx++ {
		templateSegment := templateSegments[idx]
		logicalSegment := logicalSegments[idx]

		if key, ok := metadata.TemplatePlaceholderKey(templateSegment); ok {
			if existing, exists := fields[key]; exists && existing != logicalSegment {
				return nil
			}
			fields[key] = logicalSegment
			continue
		}

		if templateSegment != logicalSegment {
			break
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func mergeTemplateFields(destination map[string]string, source map[string]string) {
	for key, value := range source {
		if _, exists := destination[key]; exists {
			continue
		}
		destination[key] = value
	}
}

func deriveFieldsFromLogicalCollectionPath(collectionPath string) map[string]string {
	segments := splitPathSegments(collectionPath)
	if len(segments) < 2 {
		return nil
	}

	derived := map[string]string{}
	for idx := 0; idx < len(segments)-1; idx++ {
		collectionSegment := strings.TrimSpace(segments[idx])
		if !looksPluralCollectionSegment(collectionSegment) {
			continue
		}

		key := singularizeCollectionToken(collectionSegment)
		value := strings.TrimSpace(segments[idx+1])
		if key == "" || value == "" {
			continue
		}
		if _, exists := derived[key]; exists {
			continue
		}
		derived[key] = value
	}

	if len(derived) == 0 {
		return nil
	}
	return derived
}

func looksPluralCollectionSegment(segment string) bool {
	lower := strings.ToLower(strings.TrimSpace(segment))
	if lower == "" {
		return false
	}
	return strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "ies")
}

func singularizeCollectionToken(token string) string {
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
	lower := strings.ToLower(last)
	if strings.HasSuffix(lower, "ies") && len(last) > 3 {
		return last[:len(last)-3] + "y"
	}
	if strings.HasSuffix(lower, "s") && len(last) > 1 {
		return last[:len(last)-1]
	}
	return last
}

func joinTemplatePaths(collectionPath string, operationPath string) string {
	base := strings.TrimSpace(collectionPath)
	if base == "" {
		base = "/"
	}
	relative := strings.TrimSpace(operationPath)
	if relative == "" {
		return base
	}

	joined := path.Join(base, relative)
	if !strings.HasPrefix(joined, "/") {
		return "/" + joined
	}
	return joined
}

func collectionPathForLogicalPath(logicalPath string) string {
	normalized := strings.TrimSpace(logicalPath)
	if normalized == "" || normalized == "/" {
		return "/"
	}

	parent := path.Dir(normalized)
	if parent == "." || parent == "" {
		return "/"
	}
	return parent
}

func splitPathSegments(value string) []string {
	return resource.SplitRawPathSegments(value)
}
