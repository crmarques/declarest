package orchestrator

import (
	"context"
	"path"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/server"
)

func (r *DefaultOrchestrator) requireRepository() (repository.ResourceStore, error) {
	if r == nil || r.Repository == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository manager is not configured", nil)
	}
	return r.Repository, nil
}

func (r *DefaultOrchestrator) requireMetadata() (metadata.MetadataService, error) {
	if r == nil || r.Metadata == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}
	return r.Metadata, nil
}

func (r *DefaultOrchestrator) requireServer() (server.ResourceServer, error) {
	if r == nil || r.Server == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "server manager is not configured", nil)
	}
	return r.Server, nil
}

func (r *DefaultOrchestrator) resolveMetadataForPath(
	ctx context.Context,
	normalizedPath string,
	allowMissing bool,
) (metadata.ResourceMetadata, error) {
	if r == nil || r.Metadata == nil {
		if allowMissing {
			return metadata.ResourceMetadata{}, nil
		}
		return metadata.ResourceMetadata{}, faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}

	resolvedMetadata, err := r.Metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if allowMissing && isTypedCategory(err, faults.NotFoundError) {
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

func isTypedCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}
