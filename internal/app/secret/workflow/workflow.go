package workflow

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
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
		if faults.IsCategory(err, faults.NotFoundError) {
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
	return DedupeAndSortAttributes(resolvedMetadata.SecretAttributes), nil
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

		fieldValue, found, err := resource.LookupJSONPointerString(payload, attribute)
		if err != nil || !found || strings.TrimSpace(fieldValue) == "" {
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
		fieldValue, found, err := resource.LookupJSONPointerString(payload, candidate)
		if err != nil || !found || strings.TrimSpace(fieldValue) == "" || IsPlaceholderValue(fieldValue) {
			continue
		}
		attributes[candidate] = struct{}{}
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
		if _, found, err := resource.LookupJSONPointer(payload, attribute); err == nil && found {
			resolvedPaths[attribute] = struct{}{}
		}
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
	secretValue, found, err := resource.LookupJSONPointerString(payload, attribute)
	if err != nil || !found || strings.TrimSpace(secretValue) == "" {
		return nil
	}
	if IsPlaceholderValue(secretValue) {
		return nil
	}

	secretKey := BuildPathScopedSecretKey(logicalPath, attribute)
	if err := secretProvider.Store(ctx, secretKey, secretValue); err != nil {
		return err
	}

	_, err = resource.SetJSONPointerValue(payload, attribute, PlaceholderValue())
	return err
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
		if !faults.IsCategory(err, faults.NotFoundError) {
			return err
		}
		currentMetadata = metadatadomain.ResourceMetadata{}
	}

	currentMetadata.SecretAttributes = MergeAttributes(
		currentMetadata.SecretAttributes,
		detected,
	)

	return metadataService.Set(ctx, logicalPath, currentMetadata)
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
		currentValue, found, err := resource.LookupJSONPointer(payload, attributePath)
		if err != nil || !found || currentValue == nil {
			continue
		}
		if stringValue, ok := currentValue.(string); ok && IsPlaceholderValue(stringValue) {
			continue
		}
		_, _ = resource.SetJSONPointerValue(payload, attributePath, PlaceholderValue())
	}
}
