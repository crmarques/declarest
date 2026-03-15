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

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestLocalResourceRepositorySaveResourceWithArtifactsWritesSidecarFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	err := repo.SaveResourceWithArtifacts(
		context.Background(),
		"/customers/acme",
		resource.Content{
			Value: map[string]any{"script": "{{include script.sh}}"},
			Descriptor: resource.PayloadDescriptor{
				Extension: ".yaml",
			},
		},
		[]repository.ResourceArtifact{
			{File: "script.sh", Content: []byte("echo hello")},
		},
	)
	if err != nil {
		t.Fatalf("SaveResourceWithArtifacts returned error: %v", err)
	}

	resourceData, err := os.ReadFile(filepath.Join(root, "customers", "acme", "resource.yaml"))
	if err != nil {
		t.Fatalf("failed to read saved resource: %v", err)
	}
	if !strings.Contains(string(resourceData), "script.sh") {
		t.Fatalf("expected placeholder in resource file, got %q", string(resourceData))
	}

	artifactData, err := repo.ReadResourceArtifact(context.Background(), "/customers/acme", "script.sh")
	if err != nil {
		t.Fatalf("ReadResourceArtifact returned error: %v", err)
	}
	if string(artifactData) != "echo hello" {
		t.Fatalf("unexpected artifact contents %q", string(artifactData))
	}
}

func TestLocalResourceRepositorySaveResourceWithArtifactsRejectsTraversal(t *testing.T) {
	t.Parallel()

	repo := NewLocalResourceRepository(t.TempDir())
	err := repo.SaveResourceWithArtifacts(
		context.Background(),
		"/customers/acme",
		resource.Content{
			Value: map[string]any{"script": "{{include script.sh}}"},
		},
		[]repository.ResourceArtifact{
			{File: "../script.sh", Content: []byte("echo hello")},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must stay within the resource directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLocalResourceRepositorySaveResourceWithArtifactsRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "customers", "acme"), 0o755); err != nil {
		t.Fatalf("failed to create resource directory: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "customers", "acme", "scripts")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	repo := NewLocalResourceRepository(root)
	err := repo.SaveResourceWithArtifacts(
		context.Background(),
		"/customers/acme",
		resource.Content{
			Value: map[string]any{"script": "{{include scripts/script.sh}}"},
		},
		[]repository.ResourceArtifact{
			{File: "scripts/script.sh", Content: []byte("echo hello")},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "escapes repository base directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
