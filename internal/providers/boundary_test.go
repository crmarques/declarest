package providers

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvidersDoNotImportSiblingProviderPackages(t *testing.T) {
	t.Parallel()

	const (
		modulePrefix    = "github.com/crmarques/declarest/"
		providersPrefix = modulePrefix + "internal/providers/"
		sharedPrefix    = providersPrefix + "shared/"
	)

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		normalizedPath := filepath.ToSlash(path)
		if !strings.HasPrefix(normalizedPath, "internal/providers/") {
			return nil
		}

		packageDir := filepath.ToSlash(filepath.Dir(normalizedPath))
		packageImportPath := modulePrefix + packageDir

		parsedFile, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imported := range parsedFile.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if !strings.HasPrefix(importPath, providersPrefix) {
				continue
			}
			if strings.HasPrefix(importPath, sharedPrefix) {
				continue
			}
			if importPath == packageImportPath {
				continue
			}

			t.Fatalf("forbidden provider import %q in %s", importPath, normalizedPath)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
