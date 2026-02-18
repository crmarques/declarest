package fsmetadata

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/support/fsutil"
)

func (s *FSMetadataService) metadataFilePath(selector string, kind metadataPathKind) (string, error) {
	if strings.TrimSpace(s.baseDir) == "" {
		return "", validationError("metadata base directory must not be empty", nil)
	}

	relativeSelector := strings.TrimPrefix(selector, "/")

	var targetPath string
	switch kind {
	case metadataPathCollection:
		if relativeSelector == "" {
			targetPath = filepath.Join(s.baseDir, "_", "metadata"+s.extension)
		} else {
			targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector), "_", "metadata"+s.extension)
		}
	case metadataPathResource:
		if relativeSelector == "" {
			return "", validationError("resource metadata path must not target root", nil)
		}
		targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector), "metadata"+s.extension)
	default:
		return "", internalError("unsupported metadata path kind", nil)
	}

	if !isPathUnderRoot(s.baseDir, targetPath) {
		return "", validationError("metadata path escapes metadata base directory", nil)
	}
	return targetPath, nil
}

func (s *FSMetadataService) selectorDirPath(selector string) (string, error) {
	if strings.TrimSpace(s.baseDir) == "" {
		return "", validationError("metadata base directory must not be empty", nil)
	}

	relativeSelector := strings.TrimPrefix(selector, "/")
	targetPath := s.baseDir
	if relativeSelector != "" {
		targetPath = filepath.Join(s.baseDir, filepath.FromSlash(relativeSelector))
	}
	if !isPathUnderRoot(s.baseDir, targetPath) {
		return "", validationError("metadata path escapes metadata base directory", nil)
	}
	return targetPath, nil
}

func parseMetadataPath(logicalPath string) (string, metadataPathKind, error) {
	value := strings.TrimSpace(logicalPath)
	if value == "" {
		return "", metadataPathResource, validationError("metadata path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", metadataPathResource, validationError("metadata path must be absolute", nil)
	}
	trailingCollectionMarker := strings.HasSuffix(normalizedInput, "/")

	rawSegments := strings.Split(normalizedInput, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return "", metadataPathResource, validationError("metadata path must not contain traversal segments", nil)
		}
		segments = append(segments, segment)
	}

	kind := metadataPathResource
	if len(segments) > 0 && segments[len(segments)-1] == "_" {
		kind = metadataPathCollection
		segments = segments[:len(segments)-1]
	}
	if trailingCollectionMarker {
		kind = metadataPathCollection
	}

	for _, segment := range segments {
		if segment == "_" {
			// "_" is used as an intermediary wildcard selector in repository templates.
			kind = metadataPathCollection
			continue
		}
		if hasWildcardPattern(segment) {
			if _, err := path.Match(segment, "sample"); err != nil {
				return "", metadataPathResource, validationError("metadata path contains invalid wildcard expression", err)
			}
			kind = metadataPathCollection
		}
	}

	selector := "/"
	if len(segments) > 0 {
		selector = "/" + strings.Join(segments, "/")
	}
	selector = path.Clean(selector)
	if !strings.HasPrefix(selector, "/") {
		return "", metadataPathResource, validationError("metadata path must be absolute", nil)
	}
	if selector != "/" {
		selector = strings.TrimSuffix(selector, "/")
	}

	if kind == metadataPathResource && selector == "/" {
		return "", metadataPathResource, validationError("resource metadata path must not target root", nil)
	}

	return selector, kind, nil
}

func cleanupEmptyParents(startDir string, rootDir string) error {
	return fsutil.CleanupEmptyParents(startDir, rootDir)
}

func isPathUnderRoot(root string, candidate string) bool {
	return fsutil.IsPathUnderRoot(root, candidate)
}
