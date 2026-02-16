package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func IsPathUnderRoot(root string, candidate string) bool {
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
