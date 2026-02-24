package fsmetadata

import (
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/providers/shared/fsutil"
	metadatadomain "github.com/crmarques/declarest/metadata"
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
	pathDescriptor, err := metadatadomain.ParsePathDescriptor(logicalPath)
	if err != nil {
		return "", metadataPathResource, err
	}

	kind := metadataPathResource
	if pathDescriptor.Collection {
		kind = metadataPathCollection
	}

	if kind == metadataPathResource && pathDescriptor.Selector == "/" {
		return "", metadataPathResource, validationError("resource metadata path must not target root", nil)
	}

	return pathDescriptor.Selector, kind, nil
}

func cleanupEmptyParents(startDir string, rootDir string) error {
	return fsutil.CleanupEmptyParents(startDir, rootDir)
}

func isPathUnderRoot(root string, candidate string) bool {
	return fsutil.IsPathUnderRoot(root, candidate)
}
