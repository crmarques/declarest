package http

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
	serverdomain "github.com/crmarques/declarest/server"
)

func extractListItems(payload any) ([]any, error) {
	switch typed := payload.(type) {
	case []any:
		return typed, nil
	case map[string]any:
		items, ok := typed["items"]
		if ok {
			values, valuesOK := items.([]any)
			if !valuesOK {
				return nil, serverdomain.NewListPayloadShapeError("list response \"items\" must be an array", nil)
			}
			return values, nil
		}

		arrayFieldKeys := make([]string, 0, len(typed))
		for key, field := range typed {
			if _, fieldIsArray := field.([]any); fieldIsArray {
				arrayFieldKeys = append(arrayFieldKeys, key)
			}
		}
		sort.Strings(arrayFieldKeys)

		if len(arrayFieldKeys) == 1 {
			values, _ := typed[arrayFieldKeys[0]].([]any)
			return values, nil
		}

		if len(arrayFieldKeys) > 1 {
			return nil, serverdomain.NewListPayloadShapeError(
				fmt.Sprintf(
					"list response object is ambiguous: expected an \"items\" array or a single array field, found array fields [%s]",
					strings.Join(arrayFieldKeys, ", "),
				),
				nil,
			)
		}

		return nil, serverdomain.NewListPayloadShapeError("list response object must include an \"items\" array", nil)
	default:
		return nil, serverdomain.NewListPayloadShapeError("list response must be an array or an object with an \"items\" array", nil)
	}
}

func buildLogicalPath(collectionPath string, alias string) (string, error) {
	return resource.JoinLogicalPath(collectionPath, alias)
}

func (g *HTTPResourceServerGateway) decodeListResponse(
	ctx context.Context,
	collectionPath string,
	md metadata.ResourceMetadata,
	spec metadata.OperationSpec,
	body []byte,
) ([]resource.Resource, error) {
	payload, err := decodeJSONResponse(body)
	if err != nil {
		return nil, err
	}
	payload, err = g.applyListJQ(ctx, payload, spec.JQ)
	if err != nil {
		return nil, err
	}

	items, err := extractListItems(payload)
	if err != nil {
		return nil, err
	}

	normalizedCollectionPath, err := resource.NormalizeLogicalPath(collectionPath)
	if err != nil {
		return nil, err
	}

	seenAliases := make(map[string]struct{}, len(items))
	list := make([]resource.Resource, 0, len(items))

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, serverdomain.NewListPayloadShapeError("list payload entries must be JSON objects", nil)
		}

		normalizedPayload, err := resource.Normalize(itemMap)
		if err != nil {
			return nil, err
		}

		payloadMap, ok := normalizedPayload.(map[string]any)
		if !ok {
			return nil, serverdomain.NewListPayloadShapeError("list payload entry normalization failed", nil)
		}

		alias, remoteID, err := identity.ResolveAliasAndRemoteIDForListItem(payloadMap, md)
		if err != nil {
			return nil, err
		}
		if _, exists := seenAliases[alias]; exists {
			return nil, conflictError(fmt.Sprintf("remote list contains duplicate alias %q", alias), nil)
		}
		seenAliases[alias] = struct{}{}

		logicalPath, err := buildLogicalPath(normalizedCollectionPath, alias)
		if err != nil {
			return nil, err
		}

		list = append(list, resource.Resource{
			LogicalPath:    logicalPath,
			CollectionPath: normalizedCollectionPath,
			LocalAlias:     alias,
			RemoteID:       remoteID,
			Metadata:       md,
			Payload:        payloadMap,
		})
	}

	sort.Slice(list, func(i int, j int) bool {
		return list[i].LogicalPath < list[j].LogicalPath
	})
	return list, nil
}
