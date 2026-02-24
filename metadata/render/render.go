package render

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

// RenderResourceMetadata merges the provided metadata with the templatescope
// derived from the payload for the target logical path and returns a fully
// rendered metadata snapshot where paths are resolved and default operations are
// expanded.
func RenderResourceMetadata(
	ctx context.Context,
	logicalPath string,
	metadataValue metadata.ResourceMetadata,
	payload resource.Value,
) (metadata.ResourceMetadata, error) {
	return RenderResourceMetadataWithFormat(ctx, logicalPath, metadataValue, payload, "json")
}

// RenderResourceMetadataWithFormat renders metadata using the provided
// repository resource format for metadata template helpers such as
// {{resource_format .}}.
func RenderResourceMetadataWithFormat(
	ctx context.Context,
	logicalPath string,
	metadataValue metadata.ResourceMetadata,
	payload resource.Value,
	resourceFormat string,
) (metadata.ResourceMetadata, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}

	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}

	alias, remoteID, err := identity.ResolveAliasAndRemoteID(normalizedPath, metadataValue, normalizedPayload)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}

	defaultCollection := defaultCollectionPath(normalizedPath)
	resourceInfo := resource.Resource{
		LogicalPath:    normalizedPath,
		CollectionPath: defaultCollection,
		LocalAlias:     alias,
		RemoteID:       remoteID,
		Metadata:       metadataValue,
		Payload:        normalizedPayload,
	}

	scope, err := templatescope.BuildResourceScope(resourceInfo)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}
	scope["resourceFormat"] = metadata.NormalizeResourceFormat(resourceFormat)

	resolvedCollectionPath, err := resolveCollectionPath(metadataValue.CollectionPath, scope)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}
	scope["collectionPath"] = resolvedCollectionPath

	rendered := metadataValue
	rendered.CollectionPath = resolvedCollectionPath

	opKeys := operationKeys(metadataValue.Operations)
	renderedOperations := make(map[string]metadata.OperationSpec, len(opKeys))
	for _, key := range opKeys {
		spec, err := metadata.ResolveOperationSpecWithScope(ctx, metadataValue, metadata.Operation(key), scope)
		if err != nil {
			return metadata.ResourceMetadata{}, err
		}
		renderedOperations[key] = spec
	}
	rendered.Operations = renderedOperations

	return rendered, nil
}

func defaultCollectionPath(logicalPath string) string {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}

	parent := path.Dir(trimmed)
	if parent == "." || parent == "" {
		return "/"
	}
	return parent
}

func resolveCollectionPath(rawCollectionPath string, scope map[string]any) (string, error) {
	candidate := strings.TrimSpace(rawCollectionPath)
	if candidate == "" {
		candidate = scopeString(scope["collectionPath"])
	}
	if candidate == "" {
		return "", nil
	}

	rendered, err := metadata.RenderTemplateString("collectionPath", candidate, scope)
	if err != nil {
		return "", err
	}
	return metadata.NormalizeRenderedOperationPath(rendered), nil
}

func scopeString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func operationKeys(values map[string]metadata.OperationSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
