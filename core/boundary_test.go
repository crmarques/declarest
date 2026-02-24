package core

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnlyCoreAndProvidersImportProviderImplementations(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean("..")
	forbiddenPrefix := "github.com/crmarques/declarest/internal/providers/"

	fset := token.NewFileSet()
	err := filepath.WalkDir(repoRoot, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			if dirEntry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relPath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		normalizedRelPath := filepath.ToSlash(relPath)
		isAllowedImporter := strings.HasPrefix(normalizedRelPath, "core/") || strings.HasPrefix(normalizedRelPath, "internal/providers/")

		parsedFile, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imported := range parsedFile.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if strings.HasPrefix(importPath, forbiddenPrefix) && !isAllowedImporter {
				t.Fatalf("forbidden provider import %q in %s", importPath, normalizedRelPath)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}

func TestDomainPackagesDoNotImportInternalPackages(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean("..")
	const (
		modulePrefix         = "github.com/crmarques/declarest/"
		forbiddenInternalPkg = modulePrefix + "internal/"
	)

	domainPrefixes := []string{
		"config/",
		"faults/",
		"metadata/",
		"orchestrator/",
		"repository/",
		"resource/",
		"secrets/",
		"server/",
	}

	isDomainPath := func(relPath string) bool {
		for _, prefix := range domainPrefixes {
			if strings.HasPrefix(relPath, prefix) {
				return true
			}
		}
		return false
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(repoRoot, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			if dirEntry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relPath, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		normalizedRelPath := filepath.ToSlash(relPath)
		if !isDomainPath(normalizedRelPath) {
			return nil
		}

		parsedFile, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imported := range parsedFile.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if strings.HasPrefix(importPath, forbiddenInternalPkg) {
				t.Fatalf("domain package file %s imports internal package %q", normalizedRelPath, importPath)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
