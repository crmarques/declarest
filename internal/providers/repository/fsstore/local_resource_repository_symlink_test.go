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

package fsstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestLocalResourceRepositorySaveRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	if err := os.Symlink(outside, filepath.Join(root, "customers")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	repo := NewLocalResourceRepository(root)
	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{"name": "ACME"},
	})
	if err == nil {
		t.Fatal("expected save to reject symlink escape path")
	}
	if !strings.Contains(err.Error(), "escapes repository base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
