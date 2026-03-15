// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"fmt"
	"strings"

	"github.com/crmarques/declarest/resource"
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
			collectionSegments = append(collectionSegments, "{{"+resource.JSONPointerForObjectKey(placeholderName)+"}}")
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
		resourcePath = "/{{" + resource.JSONPointerForObjectKey(placeholderSuffix) + "}}"
	} else {
		resourcePath = resourcePath + "/{{" + resource.JSONPointerForObjectKey(placeholderSuffix) + "}}"
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
	aliasFieldName := ""
	aliasFromOpenAPIIdentity := false
	openAPIAliasCandidate := strings.TrimSpace(openAPIIdentityAttribute)
	if openAPIAliasCandidate != "" {
		if len(openAPIResourceAttributes) == 0 || attributeSetContains(openAPIResourceAttributes, openAPIAliasCandidate) {
			aliasFieldName = openAPIAliasCandidate
			aliasFromOpenAPIIdentity = true
		}
	}
	if aliasFieldName == "" {
		aliasFieldName = inferAliasFieldNameFromSchema(openAPIResourceAttributes)
	}
	if aliasFieldName == "" {
		collectionName := inferCollectionName(target)
		singularCollectionName := singularizeToken(collectionName)
		switch singularCollectionName {
		case "":
			aliasFieldName = "id"
		case "client":
			aliasFieldName = "clientId"
		default:
			aliasFieldName = singularCollectionName
		}
	}

	idFieldName := aliasFieldName
	if !aliasFromOpenAPIIdentity && attributeSetContains(openAPIResourceAttributes, "id") {
		idFieldName = "id"
	} else if strings.HasSuffix(strings.ToLower(aliasFieldName), "id") && strings.ToLower(aliasFieldName) != "id" {
		idFieldName = "id"
	}

	return idFieldName, aliasFieldName
}

func inferAliasFieldNameFromSchema(attributes map[string]struct{}) string {
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
	return resource.SplitRawPathSegments(value)
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
