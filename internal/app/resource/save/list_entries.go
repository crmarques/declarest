package save

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

type saveEntry struct {
	LogicalPath string
	Payload     resource.Value
	Descriptor  resource.PayloadDescriptor
}

func extractSaveListItems(value resource.Value) ([]any, bool, error) {
	switch typed := value.(type) {
	case []any:
		return typed, true, nil
	case map[string]any:
		itemsValue, hasItems := typed["items"]
		if !hasItems {
			return nil, false, nil
		}
		items, ok := itemsValue.([]any)
		if !ok {
			return nil, false, faults.NewValidationError(`list payload "items" must be an array`, nil)
		}
		return items, true, nil
	default:
		return nil, false, nil
	}
}

func resolveSaveEntriesForItems(
	ctx context.Context,
	deps Dependencies,
	collectionPath string,
	items []any,
) ([]saveEntry, error) {
	normalizedCollectionPath, err := resource.NormalizeLogicalPath(collectionPath)
	if err != nil {
		return nil, err
	}

	entries := make([]saveEntry, 0, len(items))
	seenPaths := make(map[string]struct{}, len(items))

	var metadataResolved bool
	var resolvedMetadata metadatadomain.ResourceMetadata

	for _, rawItem := range items {
		normalizedItem, err := resource.Normalize(rawItem)
		if err != nil {
			return nil, err
		}

		itemMap, ok := normalizedItem.(map[string]any)
		if !ok {
			return nil, faults.NewValidationError("list payload entries must be JSON objects", nil)
		}

		entry, usedResourceEntryShape, err := resolveSaveEntryFromResourceShape(itemMap)
		if err != nil {
			return nil, err
		}
		if !usedResourceEntryShape {
			if !metadataResolved {
				metadataService, metadataErr := appdeps.RequireMetadataService(deps)
				if metadataErr != nil {
					return nil, metadataErr
				}
				resolvedMetadata, metadataErr = metadataService.ResolveForPath(ctx, normalizedCollectionPath)
				if metadataErr != nil {
					if !faults.IsCategory(metadataErr, faults.NotFoundError) {
						return nil, metadataErr
					}
					resolvedMetadata = metadatadomain.ResourceMetadata{}
				}
				metadataResolved = true
			}

			alias, err := resolveSaveListItemAlias(itemMap, resolvedMetadata)
			if err != nil {
				return nil, faults.NewValidationError(
					"list item alias could not be resolved; configure metadata alias/id attributes or use --as-one-resource",
					err,
				)
			}

			logicalPath, err := buildLogicalPathForSave(normalizedCollectionPath, alias)
			if err != nil {
				return nil, err
			}
			entry = saveEntry{
				LogicalPath: logicalPath,
				Payload:     itemMap,
				Descriptor:  resource.PayloadDescriptor{},
			}
		}

		if _, exists := seenPaths[entry.LogicalPath]; exists {
			return nil, faults.NewValidationError(
				fmt.Sprintf("list payload contains duplicate resource path %q", entry.LogicalPath),
				nil,
			)
		}
		seenPaths[entry.LogicalPath] = struct{}{}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].LogicalPath < entries[j].LogicalPath
	})
	return entries, nil
}

func resolveSaveListItemAlias(
	payload map[string]any,
	md metadatadomain.ResourceMetadata,
) (string, error) {
	alias, _, err := identity.ResolveAliasAndRemoteIDForListItem(payload, md)
	if err == nil && strings.TrimSpace(alias) != "" {
		return strings.TrimSpace(alias), nil
	}

	// Fallback keeps list save usable when metadata identity attributes are absent.
	for _, candidate := range []string{
		resource.JSONPointerForObjectKey("clientId"),
		resource.JSONPointerForObjectKey("id"),
		resource.JSONPointerForObjectKey("name"),
		resource.JSONPointerForObjectKey("alias"),
		resource.JSONPointerForObjectKey("key"),
		resource.JSONPointerForObjectKey("uuid"),
		resource.JSONPointerForObjectKey("uid"),
	} {
		value, found := identity.LookupScalarAttribute(payload, candidate)
		if !found || strings.TrimSpace(value) == "" {
			continue
		}
		return strings.TrimSpace(value), nil
	}

	return "", err
}

func resolveSaveEntryFromResourceShape(item map[string]any) (saveEntry, bool, error) {
	logicalPathValue, hasLogicalPath := item["LogicalPath"]
	payloadValue, hasPayload := item["Payload"]
	if !hasLogicalPath && !hasPayload {
		return saveEntry{}, false, nil
	}
	if !hasLogicalPath || !hasPayload {
		return saveEntry{}, false, faults.NewValidationError(
			`resource list entry must include both "LogicalPath" and "Payload"`,
			nil,
		)
	}

	logicalPath, ok := logicalPathValue.(string)
	if !ok || strings.TrimSpace(logicalPath) == "" {
		return saveEntry{}, false, faults.NewValidationError(`resource list entry "LogicalPath" must be a non-empty string`, nil)
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return saveEntry{}, false, err
	}
	// payloadValue was already normalized by resolveSaveEntriesForItems.
	normalizedPayload := payloadValue

	return saveEntry{
		LogicalPath: normalizedPath,
		Payload:     normalizedPayload,
		Descriptor:  resource.PayloadDescriptor{},
	}, true, nil
}

func buildLogicalPathForSave(collectionPath string, alias string) (string, error) {
	return resource.JoinLogicalPath(collectionPath, alias)
}

func filterSaveEntriesForSkipItems(collectionPath string, entries []saveEntry, skipItems []string) []saveEntry {
	if len(entries) == 0 || len(skipItems) == 0 {
		return entries
	}

	filtered := make([]saveEntry, 0, len(entries))
	for _, entry := range entries {
		if resource.ShouldSkipCollectionItem(collectionPath, resource.Resource{LogicalPath: entry.LogicalPath}, skipItems) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
