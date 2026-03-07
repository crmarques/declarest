package fsstore

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/providers/fsutil"
)

func (r *LocalResourceRepository) resourcePayloadFilePath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", faults.NewValidationError("repository base directory must not be empty", nil)
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	filePath := filepath.Join(r.baseDir, filepath.FromSlash(relative), "resource"+r.extension)
	if !fsutil.IsPathUnderRoot(r.baseDir, filePath) {
		return "", faults.NewValidationError("logical path escapes repository base directory", nil)
	}
	return filePath, nil
}

func (r *LocalResourceRepository) payloadFilePath(logicalPath string) (string, error) {
	return r.resourcePayloadFilePath(logicalPath)
}

func (r *LocalResourceRepository) collectionDirPath(logicalPath string) (string, error) {
	if r.baseDir == "" {
		return "", faults.NewValidationError("repository base directory must not be empty", nil)
	}
	if logicalPath == "/" {
		return r.baseDir, nil
	}

	relative := strings.TrimPrefix(logicalPath, "/")
	dirPath := filepath.Join(r.baseDir, filepath.FromSlash(relative))
	if !fsutil.IsPathUnderRoot(r.baseDir, dirPath) {
		return "", faults.NewValidationError("logical path escapes repository base directory", nil)
	}
	return dirPath, nil
}

func (r *LocalResourceRepository) resourceArtifactFilePath(logicalPath string, file string) (string, error) {
	resourceDir, err := r.collectionDirPath(logicalPath)
	if err != nil {
		return "", err
	}

	relativeFile, err := normalizeArtifactRelativePath(file)
	if err != nil {
		return "", err
	}

	targetPath := filepath.Join(resourceDir, filepath.FromSlash(relativeFile))
	if !fsutil.IsPathUnderRoot(r.baseDir, targetPath) {
		return "", faults.NewValidationError("logical path escapes repository base directory", nil)
	}

	return targetPath, nil
}

func normalizeArtifactRelativePath(file string) (string, error) {
	trimmed := strings.TrimSpace(file)
	if trimmed == "" {
		return "", faults.NewValidationError("resource artifact file must not be empty", nil)
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", faults.NewValidationError("resource artifact file must stay within the resource directory", nil)
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		return "", faults.NewValidationError("resource artifact file must not be empty", nil)
	}

	for _, segment := range strings.Split(cleaned, "/") {
		if segment == ".." {
			return "", faults.NewValidationError("resource artifact file must stay within the resource directory", nil)
		}
	}

	return cleaned, nil
}
