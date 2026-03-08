package fsstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
