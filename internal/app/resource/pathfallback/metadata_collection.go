package pathfallback

import (
	"context"
	"path"
	"strings"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

// ShouldUseMetadataCollectionFallback returns true when a path that looked like
// a single-resource read should be treated as a collection based on metadata
// selector children.
func ShouldUseMetadataCollectionFallback(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	items []resource.Resource,
) bool {
	if len(items) == 0 {
		return true
	}

	collectionChildrenResolver, ok := metadataService.(metadatadomain.CollectionChildrenResolver)
	if !ok {
		return false
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil || normalizedPath == "/" {
		return false
	}

	parentPath := path.Dir(normalizedPath)
	if parentPath == "." || parentPath == "" {
		parentPath = "/"
	}
	requestedSegment := path.Base(normalizedPath)
	if strings.TrimSpace(requestedSegment) == "" || requestedSegment == "/" {
		return false
	}

	children, err := collectionChildrenResolver.ResolveCollectionChildren(ctx, parentPath)
	if err != nil {
		return false
	}
	for _, child := range children {
		if strings.TrimSpace(child) == requestedSegment {
			return true
		}
	}

	if wildcardResolver, ok := metadataService.(metadatadomain.CollectionWildcardResolver); ok {
		hasWildcard, err := wildcardResolver.HasCollectionWildcardChild(ctx, parentPath)
		if err != nil {
			return false
		}
		if hasWildcard {
			return true
		}
	}

	return false
}
