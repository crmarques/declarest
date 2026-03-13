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

func TestLocalResourceRepositorySavePreservesOpaqueKeyBytes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	err := repo.Save(context.Background(), "/projects/platform/secrets/private-key", resource.Content{
		Value:      resource.BinaryValue{Bytes: []byte("private-key-bytes")},
		Descriptor: resource.PayloadDescriptor{Extension: ".key"},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	resourceFile := filepath.Join(root, "projects", "platform", "secrets", "private-key", "resource.key")
	data, err := os.ReadFile(resourceFile)
	if err != nil {
		t.Fatalf("expected resource.key to be written: %v", err)
	}
	if string(data) != "private-key-bytes" {
		t.Fatalf("expected original key bytes, got %q", string(data))
	}

	content, err := repo.Get(context.Background(), "/projects/platform/secrets/private-key")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if content.Descriptor.Extension != ".key" {
		t.Fatalf("expected .key extension, got %q", content.Descriptor.Extension)
	}
	binaryValue, ok := content.Value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected binary value, got %T", content.Value)
	}
	if string(binaryValue.Bytes) != "private-key-bytes" {
		t.Fatalf("expected original key bytes on readback, got %q", string(binaryValue.Bytes))
	}
}

func TestLocalResourceRepositoryGetMergesDefaultsSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
labels:
  team: platform
spec:
  enabled: true
  tags:
    - default
`)
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), `
spec:
  enabled: false
`)

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
		"spec": map[string]any{
			"enabled": false,
			"tags":    []any{"default"},
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected merged payload: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositoryCollectionDefaultsDoNotMakeResourceVisible(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
spec:
  enabled: true
`)

	exists, err := repo.Exists(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Exists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected collection defaults alone to not make the resource exist")
	}

	items, err := repo.List(context.Background(), "/customers", repository.ListPolicy{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("unexpected list items: %#v", items)
	}
}

func TestLocalResourceRepositoryRejectsLegacyResourceDefaultsSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "defaults.yaml"), "spec:\n  enabled: true\n")

	_, err := repo.Get(context.Background(), "/customers/acme")
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported per-resource defaults files") {
		t.Fatalf("expected legacy defaults error message, got %v", err)
	}
}

func TestLocalResourceRepositorySaveCompactsAgainstDefaultsSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
labels:
  team: platform
spec:
  enabled: true
  tags:
    - default
`)

	err := repo.Save(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"labels": map[string]any{
				"team": "platform",
			},
			"spec": map[string]any{
				"enabled": false,
				"tags":    []any{"override"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	resourceBytes, err := os.ReadFile(filepath.Join(root, "customers", "acme", "resource.yaml"))
	if err != nil {
		t.Fatalf("expected compacted resource file: %v", err)
	}
	if string(resourceBytes) == "" {
		t.Fatal("expected non-empty compacted resource file")
	}

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
		"spec": map[string]any{
			"enabled": false,
			"tags":    []any{"override"},
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected merged payload after save: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositorySaveRemovesRedundantOverrideFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
spec:
  enabled: true
`)
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), `
spec:
  enabled: false
`)

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

	resourceBytes, err := os.ReadFile(filepath.Join(root, "customers", "acme", "resource.yaml"))
	if err != nil {
		t.Fatalf("expected explicit empty override file, got %v", err)
	}
	if string(resourceBytes) != "{}\n" {
		t.Fatalf("unexpected explicit empty override contents: %q", string(resourceBytes))
	}
}

func TestLocalResourceRepositoryDeletePreservesCollectionDefaultsSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
spec:
  enabled: true
`)
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), `
spec:
  enabled: false
`)

	if err := repo.Delete(context.Background(), "/customers/acme", repository.DeletePolicy{}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "customers", "_", "defaults.yaml")); err != nil {
		t.Fatalf("expected collection defaults sidecar to remain, got stat err %v", err)
	}
}

func TestLocalResourceRepositoryAcceptsJSONResourceWithYAMLCollectionDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
spec:
  enabled: true
`)
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.json"), `{"spec":{"enabled":false}}`)

	content, err := repo.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("expected JSON resource + YAML defaults to be accepted, got %v", err)
	}
	want := map[string]any{"spec": map[string]any{"enabled": false}}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected merged payload: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositoryGetDefaultsReturnsRawSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), `
labels:
  team: platform
`)

	content, err := repo.GetDefaults(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("GetDefaults returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
	}
	if !reflect.DeepEqual(content.Value, want) {
		t.Fatalf("unexpected defaults payload: got %#v want %#v", content.Value, want)
	}
}

func TestLocalResourceRepositorySaveDefaultsUsesCollectionResourceDescriptor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "metadata.yaml"), "resource:\n  id: '{{/id}}'\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.json"), "{\"owner\":\"platform\"}\n")

	if err := repo.SaveDefaults(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"region": "us-east-1",
		},
	}); err != nil {
		t.Fatalf("SaveDefaults returned error: %v", err)
	}

	defaultsBytes, err := os.ReadFile(filepath.Join(root, "customers", "_", "defaults.json"))
	if err != nil {
		t.Fatalf("expected defaults.json to be written in metadata selector dir: %v", err)
	}
	if !strings.Contains(string(defaultsBytes), `"region": "us-east-1"`) {
		t.Fatalf("unexpected defaults.json contents: %q", string(defaultsBytes))
	}
	if _, err := os.Stat(filepath.Join(root, "customers", "_", "defaults.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected metadata format hint to not force defaults.yaml, got stat err %v", err)
	}
}

func TestLocalResourceRepositorySaveDefaultsUsesSeparateMetadataBaseDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	metadataRoot := t.TempDir()
	repo := NewLocalResourceRepository(root, metadataRoot)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.yaml"), "owner: platform\n")

	if err := repo.SaveDefaults(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"region": "us-east-1",
		},
	}); err != nil {
		t.Fatalf("SaveDefaults returned error: %v", err)
	}

	defaultsPath := filepath.Join(metadataRoot, "customers", "_", "defaults.yaml")
	defaultsBytes, err := os.ReadFile(defaultsPath)
	if err != nil {
		t.Fatalf("expected defaults.yaml in separate metadata base dir: %v", err)
	}
	if string(defaultsBytes) != "region: us-east-1\n" {
		t.Fatalf("unexpected defaults.yaml contents: %q", string(defaultsBytes))
	}
	if _, err := os.Stat(filepath.Join(root, "customers", "_", "defaults.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected repository root to not receive defaults.yaml, got stat err %v", err)
	}

	defaultsContent, err := repo.GetDefaults(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("GetDefaults returned error: %v", err)
	}
	wantDefaults := map[string]any{"region": "us-east-1"}
	if !reflect.DeepEqual(defaultsContent.Value, wantDefaults) {
		t.Fatalf("unexpected defaults payload: got %#v want %#v", defaultsContent.Value, wantDefaults)
	}
}

func TestLocalResourceRepositorySaveDefaultsPreservesExistingDescriptor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "_", "defaults.yaml"), "region: us-east-1\n")
	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.json"), "{\"owner\":\"platform\"}\n")

	if err := repo.SaveDefaults(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"region": "us-west-2",
		},
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}); err != nil {
		t.Fatalf("SaveDefaults returned error: %v", err)
	}

	defaultsBytes, err := os.ReadFile(filepath.Join(root, "customers", "_", "defaults.yaml"))
	if err != nil {
		t.Fatalf("expected existing defaults.yaml to be preserved: %v", err)
	}
	if string(defaultsBytes) != "region: us-west-2\n" {
		t.Fatalf("unexpected defaults.yaml contents: %q", string(defaultsBytes))
	}
	if _, err := os.Stat(filepath.Join(root, "customers", "_", "defaults.json")); !os.IsNotExist(err) {
		t.Fatalf("expected existing defaults sidecar codec to be preserved, got stat err %v", err)
	}
}

func TestLocalResourceRepositorySaveDefaultsUsesResourceDescriptorWhenNonJSONYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.properties"), "owner=platform\n")

	if err := repo.SaveDefaults(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"region": "us-east-1",
		},
	}); err != nil {
		t.Fatalf("SaveDefaults returned error: %v", err)
	}

	defaultsBytes, err := os.ReadFile(filepath.Join(root, "customers", "_", "defaults.properties"))
	if err != nil {
		t.Fatalf("expected defaults.properties to be written in metadata selector dir: %v", err)
	}
	if string(defaultsBytes) != "region=us-east-1\n" {
		t.Fatalf("unexpected defaults.properties contents: %q", string(defaultsBytes))
	}
}

func TestLocalResourceRepositorySaveDefaultsRejectsUnsupportedPayloadType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := NewLocalResourceRepository(root)

	writeRepositoryTestFile(t, filepath.Join(root, "customers", "acme", "resource.key"), "private")

	err := repo.SaveDefaults(context.Background(), "/customers/acme", resource.Content{
		Value: map[string]any{
			"region": "us-east-1",
		},
	})
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestLocalResourceRepositorySaveResourceWithArtifactsRejectsReservedDefaultsName(t *testing.T) {
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
}

func writeRepositoryTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
