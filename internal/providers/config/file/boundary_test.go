package file

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFileProviderDoesNotImportSiblingProviders(t *testing.T) {
	t.Parallel()

	forbiddenPrefix := "github.com/crmarques/declarest/internal/providers/"

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
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
			if strings.HasPrefix(importPath, forbiddenPrefix) {
				t.Fatalf("forbidden provider import %q in %s", importPath, path)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
