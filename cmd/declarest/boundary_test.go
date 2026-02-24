package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclarestCmdImportBoundary(t *testing.T) {
	t.Parallel()

	allowedImports := map[string]struct{}{
		"github.com/crmarques/declarest/config":       {},
		"github.com/crmarques/declarest/core":         {},
		"github.com/crmarques/declarest/internal/cli": {},
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsedFile, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imported := range parsedFile.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if !strings.Contains(importPath, ".") {
				// Standard library import.
				continue
			}
			if _, allowed := allowedImports[importPath]; !allowed {
				t.Fatalf("forbidden import %q in %s", importPath, path)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
