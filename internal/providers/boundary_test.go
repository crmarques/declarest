// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
