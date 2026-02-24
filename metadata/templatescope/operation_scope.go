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

var pathTemplateSegmentPattern = regexp.MustCompile(`^\{\{\s*\.([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}$`)
var jqResourcePathPattern = regexp.MustCompile(`resource\(\s*"((?:[^"\\]|\\.)*)"\s*\)`)

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
		"logicalPath":    logicalPath,
		"collectionPath": collectionPath,
		"alias":          alias,
		"remoteID":       remoteID,
		"payload":        normalizedPayload,
		"value":          normalizedPayload,
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

func BuildResourceScope(resourceInfo resource.Resource) (map[string]any, error) {
	collectionPath := resourceInfo.CollectionPath
	if strings.TrimSpace(collectionPath) == "" {
		collectionPath = collectionPathForLogicalPath(resourceInfo.LogicalPath)
	}

	scope, err := BuildOperationScope(
		resourceInfo.LogicalPath,
		collectionPath,
		resourceInfo.LocalAlias,
		resourceInfo.RemoteID,
		resourceInfo.Payload,
	)
	if err != nil {
		return nil, err
	}

	payloadMap, _ := scope["payload"].(map[string]any)
	for key, value := range DerivePathTemplateFields(resourceInfo.LogicalPath, resourceInfo.Metadata) {
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
	collectionTemplate := strings.TrimSpace(md.CollectionPath)
	if collectionTemplate == "" {
		collectionTemplate = collectionPathForLogicalPath(logicalPath)
	}
	mergeTemplateFields(derived, deriveTemplateFieldsFromPathTemplate(collectionTemplate, logicalPath))
	mergeTemplateFields(derived, deriveTemplateFieldsFromJQExpression(md.JQ, logicalPath))

	operationNames := make([]string, 0, len(md.Operations))
	for operationName := range md.Operations {
		operationNames = append(operationNames, operationName)
	}
	sort.Strings(operationNames)

	for _, operationName := range operationNames {
		spec := md.Operations[operationName]
		mergeTemplateFields(derived, deriveTemplateFieldsFromJQExpression(spec.JQ, logicalPath))

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

		matches := pathTemplateSegmentPattern.FindStringSubmatch(templateSegment)
		if len(matches) == 2 {
			key := matches[1]
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
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}
