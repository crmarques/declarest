package fsstore

import (
	"os"
	"path/filepath"
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestLocalResourceRepositoryMetadataPayloadTypeSupportsJSONAndPrefersYAML(t *testing.T) {
	t.Parallel()

	metadataDir := filepath.Join(t.TempDir(), "metadata")
	repo := NewLocalResourceRepository(t.TempDir(), resource.PayloadTypeJSON, metadataDir)

	writeMetadataPayloadTypeFixture(
		t,
		filepath.Join(metadataDir, "customers", "acme", "metadata.json"),
		false,
		resource.PayloadTypeJSON,
	)

	payloadType, found, err := repo.metadataPayloadType("/customers/acme")
	if err != nil {
		t.Fatalf("metadataPayloadType returned error for json metadata: %v", err)
	}
	if !found || payloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json payload type from json metadata, got found=%v payloadType=%q", found, payloadType)
	}

	writeMetadataPayloadTypeFixture(
		t,
		filepath.Join(metadataDir, "customers", "acme", "metadata.yaml"),
		true,
		resource.PayloadTypeYAML,
	)

	payloadType, found, err = repo.metadataPayloadType("/customers/acme")
	if err != nil {
		t.Fatalf("metadataPayloadType returned error for yaml metadata: %v", err)
	}
	if !found || payloadType != resource.PayloadTypeYAML {
		t.Fatalf("expected yaml payload type to take precedence, got found=%v payloadType=%q", found, payloadType)
	}
}

func writeMetadataPayloadTypeFixture(
	t *testing.T,
	filePath string,
	useYAML bool,
	payloadType string,
) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create metadata directory %q: %v", filepath.Dir(filePath), err)
	}

	metadata := metadatadomain.ResourceMetadata{PayloadType: payloadType}
	var (
		encoded []byte
		err     error
	)
	if useYAML {
		encoded, err = metadatadomain.EncodeResourceMetadataYAML(metadata)
	} else {
		encoded, err = metadatadomain.EncodeResourceMetadataJSON(metadata, true)
	}
	if err != nil {
		t.Fatalf("failed to encode metadata fixture %q: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, encoded, 0o644); err != nil {
		t.Fatalf("failed to write metadata fixture %q: %v", filePath, err)
	}
}
