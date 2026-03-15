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

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPathUnderRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Clean("/tmp/root")
	if !IsPathUnderRoot(root, filepath.Join(root, "a", "b")) {
		t.Fatal("expected child path to be under root")
	}
	if IsPathUnderRoot(root, filepath.Clean("/tmp/other/file")) {
		t.Fatal("expected unrelated path to be outside root")
	}
}

func TestIsPathUnderRootRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	candidate := filepath.Join(linkPath, "escaped.txt")
	if IsPathUnderRoot(root, candidate) {
		t.Fatalf("expected symlinked path %q to be rejected under root %q", candidate, root)
	}
}

func TestCleanupEmptyParents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	leaf := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatalf("failed to create leaf dir: %v", err)
	}

	if err := CleanupEmptyParents(leaf, root); err != nil {
		t.Fatalf("CleanupEmptyParents returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("expected parent to be removed, got err=%v", err)
	}
}
