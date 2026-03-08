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
