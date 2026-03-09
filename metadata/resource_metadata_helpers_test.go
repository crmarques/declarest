package metadata

import "testing"

func TestResourceMetadataIsWholeResourceSecret(t *testing.T) {
	t.Parallel()

	if (ResourceMetadata{}).IsWholeResourceSecret() {
		t.Fatal("expected nil secret pointer to be false")
	}
	if (ResourceMetadata{Secret: boolPointer(false)}).IsWholeResourceSecret() {
		t.Fatal("expected explicit false secret pointer to be false")
	}
	if !(ResourceMetadata{Secret: boolPointer(true)}).IsWholeResourceSecret() {
		t.Fatal("expected explicit true secret pointer to be true")
	}
}

func TestCloneResourceMetadataClonesWholeResourceSecretPointer(t *testing.T) {
	t.Parallel()

	original := ResourceMetadata{Secret: boolPointer(true)}
	cloned := CloneResourceMetadata(original)

	if !cloned.IsWholeResourceSecret() {
		t.Fatalf("expected cloned metadata to preserve whole-resource secret, got %#v", cloned)
	}
	if cloned.Secret == original.Secret {
		t.Fatal("expected cloned secret pointer to be independent")
	}

	*cloned.Secret = false
	if !original.IsWholeResourceSecret() {
		t.Fatalf("expected original secret pointer to remain unchanged, got %#v", original)
	}
}

func TestMergeResourceMetadataOverlaysWholeResourceSecretPointer(t *testing.T) {
	t.Parallel()

	base := ResourceMetadata{Secret: boolPointer(true)}
	overlay := ResourceMetadata{Secret: boolPointer(false)}

	merged := MergeResourceMetadata(base, overlay)
	if merged.IsWholeResourceSecret() {
		t.Fatalf("expected overlay false secret to win, got %#v", merged)
	}
	if merged.Secret == overlay.Secret {
		t.Fatal("expected merged secret pointer to be cloned")
	}
	if !base.IsWholeResourceSecret() {
		t.Fatalf("expected base metadata to remain unchanged, got %#v", base)
	}
}
