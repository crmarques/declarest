package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func IsPathUnderRoot(root string, candidate string) bool {
	rootResolved, err := resolvePathWithSymlinks(root)
	if err != nil {
		return false
	}
	candidateResolved, err := resolvePathWithSymlinks(candidate)
	if err != nil {
		return false
	}
	return isPathUnderRootLexical(rootResolved, candidateResolved)
}

func CleanupEmptyParents(startDir string, rootDir string) error {
	current := filepath.Clean(startDir)
	root := filepath.Clean(rootDir)

	for {
		if current == root {
			return nil
		}
		if current == "." || current == string(filepath.Separator) {
			return nil
		}

		err := os.Remove(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && errors.Is(pathErr.Err, fs.ErrInvalid) {
				return nil
			}
			if errors.Is(err, fs.ErrExist) || strings.Contains(err.Error(), "not empty") {
				return nil
			}
			return err
		}

		current = filepath.Dir(current)
	}
}

func isPathUnderRootLexical(root string, candidate string) bool {
	rootClean := filepath.Clean(root)
	candidateClean := filepath.Clean(candidate)

	relPath, err := filepath.Rel(rootClean, candidateClean)
	if err != nil {
		return false
	}
	if relPath == ".." {
		return false
	}
	if strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// resolvePathWithSymlinks resolves any existing symlink components while
// preserving not-yet-created suffixes so callers can safely validate future
// file paths (for example, write targets).
func resolvePathWithSymlinks(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return cleaned, nil
	}

	volume := filepath.VolumeName(cleaned)
	rest := strings.TrimPrefix(cleaned, volume)
	sep := string(filepath.Separator)

	current := ""
	switch {
	case strings.HasPrefix(rest, sep):
		current = volume + sep
		rest = strings.TrimPrefix(rest, sep)
	case volume != "":
		current = volume
	}

	parts := strings.FieldsFunc(rest, func(r rune) bool {
		return r == rune(filepath.Separator)
	})
	if len(parts) == 0 {
		if current == "" {
			return cleaned, nil
		}
		return filepath.Clean(current), nil
	}

	for idx, part := range parts {
		next := filepath.Join(current, part)

		info, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Preserve the remaining (non-existent) suffix lexically after the
				// last resolved existing component.
				if idx < len(parts)-1 {
					next = filepath.Join(next, filepath.Join(parts[idx+1:]...))
				}
				return filepath.Clean(next), nil
			}
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolvedLink, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", err
			}
			current = resolvedLink
			continue
		}

		current = next
	}

	if current == "" {
		return cleaned, nil
	}
	return filepath.Clean(current), nil
}
