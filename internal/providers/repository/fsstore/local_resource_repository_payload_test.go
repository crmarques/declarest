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
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestLocalResourceRepositorySavePreservesKnownPayloadExtension(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{"name": "ACME"},
		Descriptor: resource.PayloadDescriptor{
			Extension: ".yml",
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "customers", "acme", "resource.yml")); err != nil {
		t.Fatalf("expected resource.yml to be written: %v", err)
	}

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml payload type, got %q", content.Descriptor.PayloadType)
	}
	if content.Descriptor.MediaType != "application/yaml" {
		t.Fatalf("expected application/yaml media type, got %q", content.Descriptor.MediaType)
	}
	if content.Descriptor.Extension != ".yml" {
		t.Fatalf("expected .yml extension, got %q", content.Descriptor.Extension)
	}
}

func TestLocalResourceRepositorySavePreservesUnknownPayloadExtensionAsOctetStream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: resource.BinaryValue{Bytes: []byte("abc")},
		Descriptor: resource.PayloadDescriptor{
			Extension: ".cfg",
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "customers", "acme", "resource.cfg")); err != nil {
		t.Fatalf("expected resource.cfg to be written: %v", err)
	}

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if content.Descriptor.PayloadType != resource.PayloadTypeOctetStream {
		t.Fatalf("expected octet-stream payload type, got %q", content.Descriptor.PayloadType)
	}
	if content.Descriptor.MediaType != "application/octet-stream" {
		t.Fatalf("expected octet-stream media type, got %q", content.Descriptor.MediaType)
	}
	if content.Descriptor.Extension != ".cfg" {
		t.Fatalf("expected .cfg extension, got %q", content.Descriptor.Extension)
	}
	if _, ok := content.Value.(resource.BinaryValue); !ok {
		t.Fatalf("expected binary value, got %T", content.Value)
	}
}

func TestLocalResourceRepositoryGetReturnsRawPayloadWithoutMetadataDefaultsMerge(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), "spec:\n  enabled: true\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), "spec:\n  enabled: false\n")

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	want := map[string]any{
		"spec": map[string]any{
			"enabled": false,
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected raw payload: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositoryIgnoresMetadataOwnedDefaultsFilesWhenReadingResource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "defaults.yaml"), "spec:\n  enabled: true\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "defaults-prod.json"), "{\"spec\":{\"tier\":\"prod\"}}\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), "spec:\n  enabled: false\n")

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := map[string]any{
		"spec": map[string]any{
			"enabled": false,
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected raw payload: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositoryMetadataDefaultsDoNotMakeResourceVisible(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), "spec:\n  enabled: true\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults-prod.yaml"), "spec:\n  tier: prod\n")

	exists, err := repo.Exists(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected metadata defaults alone to not make the resource exist")
	}

	items, err := repo.List(context.Background(), "/customers", repository.ListPolicy{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("unexpected list items: %#v", items)
	}
}

func TestLocalResourceRepositorySaveDoesNotCompactAgainstMetadataDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), "spec:\n  enabled: true\n")

	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"spec": map[string]any{
				"enabled": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	resourceBytes, err := os.ReadFile(filepath.Join(root, "customers", "acme", "resource.json"))
	if err != nil {
		t.Fatalf("expected raw resource file: %v", err)
	}
	if !strings.Contains(string(resourceBytes), "\"enabled\": true") {
		t.Fatalf("expected raw resource file to keep defaulted field, got %q", string(resourceBytes))
	}
}

func TestLocalResourceRepositoryDeletePreservesMetadataDefaultsArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), "spec:\n  enabled: true\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), "spec:\n  enabled: false\n")

	if err := repo.Delete(context.Background(), "/customers/acme", repository.DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "customers", "_", "defaults.yaml")); err != nil {
		t.Fatalf("expected metadata defaults artifact to remain, got stat err %v", err)
	}
}

func TestLocalResourceRepositorySaveResourceWithArtifactsRejectsReservedDefaultsPrefix(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	err := repo.SaveResourceWithArtifacts(
		context.Background(),
		"/customers/acme",
		resource.Content{
			Value: map[string]any{"name": "acme"},
		},
		[]repository.ResourceArtifact{
			{File: "defaults.yaml", Content: []byte("x")},
		},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}

	err = repo.SaveResourceWithArtifacts(
		context.Background(),
		"/customers/acme",
		resource.Content{
			Value: map[string]any{"name": "acme"},
		},
		[]repository.ResourceArtifact{
			{File: "defaults-prod.yaml", Content: []byte("x")},
		},
	)
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error for profile defaults artifact, got %v", err)
	}
}

func writeRepositoryTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
