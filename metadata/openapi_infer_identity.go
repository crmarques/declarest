package metadata

import (
	"fmt"
	"strings"
)

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

func inferIdentityAttributes(
	target inferTarget,
	openAPIIdentityAttribute string,
	openAPIResourceAttributes map[string]struct{},
) (string, string) {
	aliasAttribute := ""
	aliasFromOpenAPIIdentity := false
	openAPIAliasCandidate := strings.TrimSpace(openAPIIdentityAttribute)
	if openAPIAliasCandidate != "" {
		if len(openAPIResourceAttributes) == 0 || attributeSetContains(openAPIResourceAttributes, openAPIAliasCandidate) {
			aliasAttribute = openAPIAliasCandidate
			aliasFromOpenAPIIdentity = true
		}
	}
	if aliasAttribute == "" {
		aliasAttribute = inferAliasAttributeFromSchema(openAPIResourceAttributes)
	}
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
	if !aliasFromOpenAPIIdentity && attributeSetContains(openAPIResourceAttributes, "id") {
		idAttribute = "id"
	} else if strings.HasSuffix(strings.ToLower(aliasAttribute), "id") && strings.ToLower(aliasAttribute) != "id" {
		idAttribute = "id"
	}

	return idAttribute, aliasAttribute
}

func inferAliasAttributeFromSchema(attributes map[string]struct{}) string {
	if len(attributes) == 0 {
		return ""
	}

	for _, candidate := range []string{"clientId", "alias", "name", "id", "key", "uuid", "uid"} {
		if attributeSetContains(attributes, candidate) {
			return candidate
		}
	}

	return ""
}

func attributeSetContains(attributes map[string]struct{}, value string) bool {
	if len(attributes) == 0 {
		return false
	}

	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return false
	}

	if _, found := attributes[trimmedValue]; found {
		return true
	}

	for key := range attributes {
		if strings.EqualFold(strings.TrimSpace(key), trimmedValue) {
			return true
		}
	}

	return false
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
