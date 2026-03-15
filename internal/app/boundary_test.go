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

package app_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppLayerImportBoundaries(t *testing.T) {
	t.Parallel()

	const (
		modulePrefix             = "github.com/crmarques/declarest/"
		forbiddenCLIPrefix       = modulePrefix + "internal/cli/"
		forbiddenProvidersPrefix = modulePrefix + "internal/providers/"
	)

	forbiddenExactImports := map[string]struct{}{
		"github.com/spf13/cobra": {},
		"github.com/spf13/pflag": {},
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
		if !strings.HasPrefix(filepath.ToSlash(path), "internal/app/") {
			return nil
		}

		parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, "\"")
			if strings.HasPrefix(importPath, forbiddenCLIPrefix) {
				t.Fatalf("internal/app must not import CLI packages: %q in %s", importPath, path)
			}
			if strings.HasPrefix(importPath, forbiddenProvidersPrefix) {
				t.Fatalf("internal/app must not import provider packages: %q in %s", importPath, path)
			}
			if _, forbidden := forbiddenExactImports[importPath]; forbidden {
				t.Fatalf("internal/app must not import CLI framework packages: %q in %s", importPath, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("boundary scan failed: %v", err)
	}
}
