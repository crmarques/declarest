package resource

import (
	"path"
	"strings"
)

func FilterCollectionItems(collectionPath string, items []Resource, excluded []string) []Resource {
	if len(items) == 0 || len(excluded) == 0 {
		return items
	}

	set := skipItemSet(excluded)
	if len(set) == 0 {
		return items
	}

	filtered := make([]Resource, 0, len(items))
	for _, item := range items {
		if shouldSkipCollectionItem(collectionPath, item, set) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func ShouldSkipCollectionItem(collectionPath string, item Resource, excluded []string) bool {
	set := skipItemSet(excluded)
	return shouldSkipCollectionItem(collectionPath, item, set)
}

func shouldSkipCollectionItem(collectionPath string, item Resource, set map[string]struct{}) bool {
	if len(set) == 0 {
		return false
	}

	if skipSetContains(set, item.LogicalPath) || skipSetContains(set, item.LocalAlias) || skipSetContains(set, item.RemoteID) {
		return true
	}

	if childSegment, ok := ChildSegment(collectionPath, item.LogicalPath); ok && skipSetContains(set, childSegment) {
		return true
	}

	normalizedPath, err := NormalizeLogicalPath(item.LogicalPath)
	if err == nil && skipSetContains(set, path.Base(normalizedPath)) {
		return true
	}

	return false
}

func skipItemSet(excluded []string) map[string]struct{} {
	if len(excluded) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(excluded))
	for _, item := range excluded {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func skipSetContains(set map[string]struct{}, value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	_, found := set[trimmed]
	return found
}
