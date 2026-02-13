package cli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIDoesNotImportAdapterImplementations(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"github.com/crmarques/declarest/internal/adapters/repository/fs",
		"github.com/crmarques/declarest/internal/adapters/repository/git",
		"github.com/crmarques/declarest/internal/adapters/server/http",
		"github.com/crmarques/declarest/internal/adapters/server/openapi",
		"github.com/crmarques/declarest/internal/adapters/secrets/file",
		"github.com/crmarques/declarest/adapters",
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if strings.Contains(importPath, "/noop") {
				t.Fatalf("forbidden import %q in %s", importPath, path)
			}
			for _, prefix := range forbiddenPrefixes {
				if strings.HasPrefix(importPath, prefix) {
					t.Fatalf("forbidden import %q in %s", importPath, path)
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
