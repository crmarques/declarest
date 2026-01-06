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
	return ResourceFileRelPathForFormat(path, ResourceFormatJSON)
}

func ResourceFileRelPathForFormat(path string, format ResourceFormat) string {
	trimmed := strings.Trim(path, " /")
	fileName := resourceFileNameForFormat(format)
	if trimmed == "" {
		return filepath.Join(fileName)
	}
	return filepath.Join(filepath.FromSlash(trimmed), fileName)
}

func ResourceDirRelPath(path string) string {
	trimmed := strings.Trim(path, " /")
	if trimmed == "" {
		return "."
	}
	return filepath.FromSlash(trimmed)
}

type resourceFileCandidate struct {
	relPath string
	format  ResourceFormat
}

func resourceFileRelPathCandidates(path string, format ResourceFormat) []resourceFileCandidate {
	trimmed := strings.Trim(path, " /")
	var base string
	if trimmed == "" {
		base = "."
	} else {
		base = filepath.FromSlash(trimmed)
	}

	candidates := make([]resourceFileCandidate, 0, 3)
	add := func(name string, format ResourceFormat) {
		if base == "." {
			candidates = append(candidates, resourceFileCandidate{
				relPath: filepath.Join(name),
				format:  format,
			})
			return
		}
		candidates = append(candidates, resourceFileCandidate{
			relPath: filepath.Join(base, name),
			format:  format,
		})
	}

	switch normalizeResourceFormat(format) {
	case ResourceFormatYAML:
		add(resourceFileYAML, ResourceFormatYAML)
		add(resourceFileYML, ResourceFormatYAML)
		add(resourceFileJSON, ResourceFormatJSON)
	default:
		add(resourceFileJSON, ResourceFormatJSON)
		add(resourceFileYAML, ResourceFormatYAML)
		add(resourceFileYML, ResourceFormatYAML)
	}

	return candidates
}

func resourceFileRelPathsAll(path string) []string {
	trimmed := strings.Trim(path, " /")
	var base string
	if trimmed == "" {
		base = "."
	} else {
		base = filepath.FromSlash(trimmed)
	}

	names := []string{resourceFileJSON, resourceFileYAML, resourceFileYML}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		if base == "." {
			paths = append(paths, filepath.Join(name))
		} else {
			paths = append(paths, filepath.Join(base, name))
		}
	}
	return paths
}

func isResourceFileName(name string) bool {
	switch name {
	case resourceFileJSON, resourceFileYAML, resourceFileYML:
		return true
	default:
		return false
	}
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
