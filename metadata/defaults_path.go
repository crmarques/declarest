package metadata

import (
	"fmt"
	"strings"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"
)

type CollectionPathMode int

const (
	CollectionPathRuntime CollectionPathMode = iota
	CollectionPathTemplate
)

func DefaultMetadataForPath(path string, mode CollectionPathMode) (resource.ResourceMetadata, bool) {
	trimmed := strings.TrimSpace(path)
	isCollection := strings.HasSuffix(trimmed, "/")
	clean := strings.Trim(trimmed, " /")
	segments := resource.SplitPathSegments(clean)
	collectionSegments := segments
	if !isCollection && len(collectionSegments) > 0 {
		collectionSegments = collectionSegments[:len(collectionSegments)-1]
	}

	meta := DefaultMetadata(collectionSegments)
	if mode == CollectionPathTemplate && meta.ResourceInfo != nil {
		meta.ResourceInfo.CollectionPath = templateCollectionPath(collectionSegments)
	}

	return meta, isCollection
}

func ApplyOpenAPIDefaults(meta resource.ResourceMetadata, logicalPath string, isCollection bool, data resource.Resource, spec *openapi.Spec) resource.ResourceMetadata {
	if spec == nil {
		return meta
	}
	targetPath := strings.TrimSpace(logicalPath)
	if isCollection {
		if meta.ResourceInfo != nil {
			if coll := strings.TrimSpace(meta.ResourceInfo.CollectionPath); coll != "" {
				targetPath = coll
			}
		}
	} else {
		record := resource.ResourceRecord{
			Path: logicalPath,
			Meta: meta,
			Data: data,
		}
		if remotePath := strings.TrimSpace(record.RemoteResourcePath(data)); remotePath != "" {
			targetPath = remotePath
		}
	}
	return openapi.ApplyDefaults(meta, targetPath, isCollection, spec)
}

func templateCollectionPath(segments []string) string {
	normalized := make([]string, 0, len(segments))
	for _, segment := range segments {
		if trimmed := strings.TrimSpace(segment); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	if len(normalized) == 0 {
		return "/"
	}
	if len(normalized) == 1 {
		return resource.NormalizePath("/" + normalized[0])
	}
	return fmt.Sprintf("{{../resourceInfo.collectionPath}}/%s", normalized[len(normalized)-1])
}
