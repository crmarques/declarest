package templatescope

import (
	"strings"

	"github.com/crmarques/declarest/resource"
)

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
