package resource

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func resolveMetadataForSecretCheck(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
) (metadatadomain.ResourceMetadata, error) {
	if deps.Metadata == nil {
		return metadatadomain.ResourceMetadata{}, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	resolvedMetadata, err := deps.Metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if isTypedErrorCategory(err, faults.NotFoundError) {
			return metadatadomain.ResourceMetadata{}, nil
		}
		return metadatadomain.ResourceMetadata{}, err
	}
	return resolvedMetadata, nil
}

func dedupeAndSortSaveSecretAttributes(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func findAttributeParentMap(payload map[string]any, attribute string) (map[string]any, string, bool) {
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

func secretPlaceholderValue() string {
	return "{{secret .}}"
}

func isSecretPlaceholderValue(value string) bool {
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
