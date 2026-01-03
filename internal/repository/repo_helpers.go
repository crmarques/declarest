package repository

import (
	"fmt"
	"path/filepath"
	"strings"

	"declarest/internal/resource"
)

func AbsBaseDir(baseDir string) (string, error) {
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		dir = "."
	}
	dir = filepath.Clean(dir)
	return filepath.Abs(dir)
}

func SafeJoin(baseDir, rel string) (string, error) {
	base, err := AbsBaseDir(baseDir)
	if err != nil {
		return "", err
	}
	full := filepath.Join(base, rel)
	full = filepath.Clean(full)
	relPath, err := filepath.Rel(base, full)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base directory %q", full, base)
	}
	return full, nil
}

func ResourceFileRelPath(path string) string {
	trimmed := strings.Trim(path, " /")
	if trimmed == "" {
		return filepath.Join("resource.json")
	}
	return filepath.Join(filepath.FromSlash(trimmed), "resource.json")
}

func ResourceDirRelPath(path string) string {
	trimmed := strings.Trim(path, " /")
	if trimmed == "" {
		return "."
	}
	return filepath.FromSlash(trimmed)
}

func MetadataFileRelPath(path string) string {
	trimmed := strings.TrimSpace(path)
	isCollection := resource.IsCollectionPath(trimmed)
	clean := strings.Trim(trimmed, " /")

	if isCollection {
		if clean == "" {
			return filepath.Join("_", "metadata.json")
		}
		return filepath.Join(filepath.FromSlash(clean), "_", "metadata.json")
	}

	if clean == "" {
		return filepath.Join("metadata.json")
	}
	return filepath.Join(filepath.FromSlash(clean), "metadata.json")
}
