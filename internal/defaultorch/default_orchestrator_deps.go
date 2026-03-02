package defaultorch

import (
	"context"
	"path"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/gateway"
)

func (r *DefaultOrchestrator) requireRepository() (repository.ResourceStore, error) {
	if r == nil || r.repository == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository store is not configured", nil)
	}
	return r.repository, nil
}

func (r *DefaultOrchestrator) requireServer() (gateway.ResourceGateway, error) {
	if r == nil || r.server == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "resource server is not configured", nil)
	}
	return r.server, nil
}

func (r *DefaultOrchestrator) resolveMetadataForPath(
	ctx context.Context,
	normalizedPath string,
	allowMissing bool,
) (metadata.ResourceMetadata, error) {
	if r == nil || r.metadata == nil {
		if allowMissing {
			return metadata.ResourceMetadata{}, nil
		}
		return metadata.ResourceMetadata{}, faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}

	resolvedMetadata, err := r.metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if allowMissing && faults.IsCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "metadata missing for path=%q; using empty metadata fallback", normalizedPath)
			return metadata.ResourceMetadata{}, nil
		}
		return metadata.ResourceMetadata{}, err
	}

	return resolvedMetadata, nil
}

func collectionPathFor(normalizedPath string) string {
	if normalizedPath == "/" {
		return "/"
	}

	collectionPath := path.Dir(normalizedPath)
	if collectionPath == "." || collectionPath == "" {
		return "/"
	}

	return collectionPath
}

func isDirectChildPath(parentPath string, logicalPath string) bool {
	if parentPath == "/" {
		return len(splitLogicalPathSegments(logicalPath)) == 1
	}

	parentSegments := splitLogicalPathSegments(parentPath)
	childSegments := splitLogicalPathSegments(logicalPath)
	if len(childSegments) != len(parentSegments)+1 {
		return false
	}

	for idx := range parentSegments {
		if parentSegments[idx] != childSegments[idx] {
			return false
		}
	}
	return true
}

func splitLogicalPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
