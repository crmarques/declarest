package templatescope

import (
	"regexp"
	"sort"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

var pathTemplateSegmentPattern = regexp.MustCompile(`^\{\{\s*\.([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}$`)

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
	return BuildOperationScope(
		resourceInfo.LogicalPath,
		resourceInfo.CollectionPath,
		resourceInfo.LocalAlias,
		resourceInfo.RemoteID,
		resourceInfo.Payload,
	)
}

func DerivePathTemplateFields(logicalPath string, md metadata.ResourceMetadata) map[string]string {
	derived := map[string]string{}
	if md.Operations == nil {
		return derived
	}

	operationNames := make([]string, 0, len(md.Operations))
	for operationName := range md.Operations {
		operationNames = append(operationNames, operationName)
	}
	sort.Strings(operationNames)

	for _, operationName := range operationNames {
		spec := md.Operations[operationName]
		fields := deriveTemplateFieldsFromPathTemplate(spec.Path, logicalPath)
		for key, value := range fields {
			if _, exists := derived[key]; exists {
				continue
			}
			derived[key] = value
		}
	}

	return derived
}

func deriveTemplateFieldsFromPathTemplate(pathTemplate string, logicalPath string) map[string]string {
	if !strings.Contains(pathTemplate, "{{") {
		return nil
	}

	templateSegments := splitPathSegments(pathTemplate)
	logicalSegments := splitPathSegments(logicalPath)
	if len(templateSegments) == 0 || len(templateSegments) != len(logicalSegments) {
		return nil
	}

	fields := make(map[string]string)
	for idx, templateSegment := range templateSegments {
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
			return nil
		}
	}

	return fields
}

func splitPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}
