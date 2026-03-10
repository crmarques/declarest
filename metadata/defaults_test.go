package metadata

import "testing"

func TestDefaultResourceMetadataDefaultsIdentityToIDPointer(t *testing.T) {
	t.Parallel()

	defaults := DefaultResourceMetadata()
	if defaults.ID != "/id" {
		t.Fatalf("expected default resource.id=/id, got %q", defaults.ID)
	}
	if defaults.Alias != "/id" {
		t.Fatalf("expected default resource.alias=/id, got %q", defaults.Alias)
	}
}
