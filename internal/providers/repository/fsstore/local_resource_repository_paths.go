package fsstore

import (
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/providers/shared/fsutil"
)

func (r *LocalResourceRepository) resourcePayloadFilePath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", validationError("repository base directory must not be empty", nil)
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	filePath := filepath.Join(r.baseDir, filepath.FromSlash(relative), "resource"+r.extension)
	if !fsutil.IsPathUnderRoot(r.baseDir, filePath) {
		return "", validationError("logical path escapes repository base directory", nil)
	}
	return filePath, nil
}

func (r *LocalResourceRepository) payloadFilePath(logicalPath string) (string, error) {
	return r.resourcePayloadFilePath(logicalPath)
}

func (r *LocalResourceRepository) legacyPayloadFilePath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", validationError("repository base directory must not be empty", nil)
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	filePath := filepath.Join(r.baseDir, filepath.FromSlash(relative+r.extension))
	if !fsutil.IsPathUnderRoot(r.baseDir, filePath) {
		return "", validationError("logical path escapes repository base directory", nil)
	}
	return filePath, nil
}

func (r *LocalResourceRepository) collectionDirPath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", validationError("repository base directory must not be empty", nil)
	}
	if logicalPath == "/" {
		return r.baseDir, nil
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	dirPath := filepath.Join(r.baseDir, filepath.FromSlash(relative))
	if !fsutil.IsPathUnderRoot(r.baseDir, dirPath) {
		return "", validationError("logical path escapes repository base directory", nil)
	}
	return dirPath, nil
}
