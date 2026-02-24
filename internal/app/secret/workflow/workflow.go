package workflow

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func ResolveMetadataForSecretCheck(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
) (metadatadomain.ResourceMetadata, error) {
	if metadataService == nil {
		return metadatadomain.ResourceMetadata{}, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	resolvedMetadata, err := metadataService.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if isTypedErrorCategory(err, faults.NotFoundError) {
			return metadatadomain.ResourceMetadata{}, nil
		}
		return metadatadomain.ResourceMetadata{}, err
	}
	return resolvedMetadata, nil
}

func ResolveDeclaredAttributes(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
) ([]string, error) {
	resolvedMetadata, err := ResolveMetadataForSecretCheck(ctx, metadataService, logicalPath)
	if err != nil {
		return nil, err
	}
	return DedupeAndSortAttributes(resolvedMetadata.SecretsFromAttributes), nil
}

func DedupeAndSortAttributes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func MergeAttributes(existing []string, detected []string) []string {
	merged := make([]string, 0, len(existing)+len(detected))
	merged = append(merged, existing...)
	merged = append(merged, detected...)
	return DedupeAndSortAttributes(merged)
}

func DetectMetadataSecretCandidates(value resource.Value, attributes []string) []string {
	payload, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	candidates := make([]string, 0)
	seenAttributes := make(map[string]struct{})
	for _, rawAttribute := range attributes {
		attribute := strings.TrimSpace(rawAttribute)
		if attribute == "" {
			continue
		}
		if _, seen := seenAttributes[attribute]; seen {
			continue
		}
		seenAttributes[attribute] = struct{}{}

		fieldValue, found := identity.LookupScalarAttribute(payload, attribute)
		if !found || strings.TrimSpace(fieldValue) == "" {
			continue
		}
		if IsPlaceholderValue(fieldValue) {
			continue
		}
		if !IsLikelyPlaintextValue(fieldValue) {
			continue
		}
		candidates = append(candidates, attribute)
	}

	sort.Strings(candidates)
	return candidates
}

func ResolveAttributePathsForCandidates(payload map[string]any, candidates []string) []string {
	attributes := make(map[string]struct{})
	for _, rawCandidate := range candidates {
		candidate := strings.TrimSpace(rawCandidate)
		if candidate == "" {
			continue
		}

		if strings.Contains(candidate, ".") {
			fieldValue, found := identity.LookupScalarAttribute(payload, candidate)
			if found && strings.TrimSpace(fieldValue) != "" && !IsPlaceholderValue(fieldValue) {
				attributes[candidate] = struct{}{}
			}
			continue
		}

		collectCandidateAttributePaths(payload, "", candidate, attributes)
	}

	result := make([]string, 0, len(attributes))
	for attribute := range attributes {
		result = append(result, attribute)
	}
	sort.Strings(result)
	return result
}

func ResolveAttributePaths(payload map[string]any, secretAttributes []string) []string {
	resolvedPaths := make(map[string]struct{})
	for _, rawAttribute := range secretAttributes {
		attribute := strings.TrimSpace(rawAttribute)
		if attribute == "" {
			continue
		}
		if strings.Contains(attribute, ".") {
			if _, _, found := FindAttributeParentMap(payload, attribute); found {
				resolvedPaths[attribute] = struct{}{}
			}
			continue
		}
		collectAttributePaths(payload, "", attribute, resolvedPaths)
	}

	paths := make([]string, 0, len(resolvedPaths))
	for attributePath := range resolvedPaths {
		paths = append(paths, attributePath)
	}
	sort.Strings(paths)
	return paths
}

func MaskValue(value resource.Value, secretAttributes []string) (resource.Value, error) {
	normalizedValue, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	switch typed := normalizedValue.(type) {
	case map[string]any:
		maskPayload(typed, secretAttributes)
		return typed, nil
	case []any:
		items := make([]any, len(typed))
		for idx := range typed {
			entry := typed[idx]
			entryPayload, ok := entry.(map[string]any)
			if !ok {
				items[idx] = entry
				continue
			}
			maskPayload(entryPayload, secretAttributes)
			items[idx] = entryPayload
		}
		return items, nil
	default:
		return normalizedValue, nil
	}
}

func StoreAndMaskAttribute(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	payload map[string]any,
	logicalPath string,
	attribute string,
) error {
	secretValue, found := identity.LookupScalarAttribute(payload, attribute)
	if !found || strings.TrimSpace(secretValue) == "" {
		return nil
	}
	if IsPlaceholderValue(secretValue) {
		return nil
	}

	parent, leafKey, found := FindAttributeParentMap(payload, attribute)
	if !found {
		return nil
	}

	secretKey := BuildPathScopedSecretKey(logicalPath, attribute)
	if err := secretProvider.Store(ctx, secretKey, secretValue); err != nil {
		return err
	}

	parent[leafKey] = PlaceholderValue()
	return nil
}

func PersistDetectedAttributes(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	detected []string,
) error {
	if len(detected) == 0 {
		return nil
	}
	if metadataService == nil {
		return faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}

	currentMetadata, err := metadataService.Get(ctx, logicalPath)
	if err != nil {
		if !isTypedErrorCategory(err, faults.NotFoundError) {
			return err
		}
		currentMetadata = metadatadomain.ResourceMetadata{}
	}

	currentMetadata.SecretsFromAttributes = MergeAttributes(
		currentMetadata.SecretsFromAttributes,
		detected,
	)

	return metadataService.Set(ctx, logicalPath, currentMetadata)
}

func FindAttributeParentMap(payload map[string]any, attribute string) (map[string]any, string, bool) {
	segments := strings.Split(strings.TrimSpace(attribute), ".")
	if len(segments) == 0 {
		return nil, "", false
	}

	current := payload
	for idx := 0; idx < len(segments)-1; idx++ {
		segment := strings.TrimSpace(segments[idx])
		if segment == "" {
			return nil, "", false
		}

		nextRaw, exists := current[segment]
		if !exists {
			return nil, "", false
		}
		next, ok := nextRaw.(map[string]any)
		if !ok {
			return nil, "", false
		}
		current = next
	}

	leafKey := strings.TrimSpace(segments[len(segments)-1])
	if leafKey == "" {
		return nil, "", false
	}
	if _, exists := current[leafKey]; !exists {
		return nil, "", false
	}

	return current, leafKey, true
}

func PlaceholderValue() string {
	return "{{secret .}}"
}

func BuildPathScopedSecretKey(logicalPath string, attribute string) string {
	return strings.TrimSpace(logicalPath) + ":" + strings.TrimSpace(attribute)
}

func IsPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}")
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "secret") {
		return false
	}

	argument := strings.TrimSpace(strings.TrimPrefix(inner, "secret"))
	if argument == "." {
		return true
	}
	if strings.HasPrefix(argument, "\"") {
		parsed, err := strconv.Unquote(argument)
		if err != nil {
			return false
		}
		return strings.TrimSpace(parsed) != ""
	}
	if strings.ContainsAny(argument, " \t\r\n") {
		return false
	}
	return strings.TrimSpace(argument) != ""
}

func IsLikelyPlaintextValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if isNumericOnlyString(trimmed) {
		return false
	}

	switch strings.ToLower(trimmed) {
	case "true", "false", "yes", "no", "on", "off", "enabled", "disabled":
		return false
	default:
		return true
	}
}

func isNumericOnlyString(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	for _, symbol := range trimmed {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return true
}

func maskPayload(payload map[string]any, secretAttributes []string) {
	paths := ResolveAttributePaths(payload, secretAttributes)
	for _, attributePath := range paths {
		parent, leafKey, found := FindAttributeParentMap(payload, attributePath)
		if !found {
			continue
		}

		currentValue := parent[leafKey]
		if currentValue == nil {
			continue
		}
		if stringValue, ok := currentValue.(string); ok && IsPlaceholderValue(stringValue) {
			continue
		}
		parent[leafKey] = PlaceholderValue()
	}
}

func collectAttributePaths(
	value any,
	prefix string,
	attribute string,
	paths map[string]struct{},
) {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			currentPath := key
			if prefix != "" {
				currentPath = prefix + "." + key
			}
			if key == attribute {
				paths[currentPath] = struct{}{}
			}
			collectAttributePaths(field, currentPath, attribute, paths)
		}
	case []any:
		// Arrays are intentionally ignored because metadata secret paths are map-path based.
		return
	}
}

func collectCandidateAttributePaths(
	value any,
	prefix string,
	candidate string,
	attributes map[string]struct{},
) {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			attribute := key
			if prefix != "" {
				attribute = prefix + "." + key
			}
			if key == candidate {
				fieldValue, ok := field.(string)
				if ok && strings.TrimSpace(fieldValue) != "" && !IsPlaceholderValue(fieldValue) {
					attributes[attribute] = struct{}{}
				}
			}
			collectCandidateAttributePaths(field, attribute, candidate, attributes)
		}
	case []any:
		// Arrays are intentionally skipped because metadata attributes are map-path based.
		return
	}
}

func isTypedErrorCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}
