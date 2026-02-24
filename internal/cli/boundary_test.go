package cli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIDoesNotImportProviderImplementations(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"github.com/crmarques/declarest/internal/providers/",
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

func TestCLIImportsFollowAllowedProjectBoundaries(t *testing.T) {
	t.Parallel()

	const modulePrefix = "github.com/crmarques/declarest/"

	allowedPrefixes := []string{
		modulePrefix + "internal/cli/",
		modulePrefix + "internal/app/",
		modulePrefix + "config",
		modulePrefix + "faults",
		modulePrefix + "metadata",
		modulePrefix + "orchestrator",
		modulePrefix + "repository",
		modulePrefix + "resource",
		modulePrefix + "secrets",
		modulePrefix + "server",
	}

	allowedExactImports := map[string]struct{}{
		modulePrefix + "debugctx": {},
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
			if !strings.HasPrefix(importPath, modulePrefix) {
				continue
			}
			if _, allowed := allowedExactImports[importPath]; allowed {
				continue
			}

			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(importPath, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Fatalf("forbidden project import %q in %s", importPath, path)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
