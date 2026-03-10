package metadata

import (
	"reflect"
	"testing"
)

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

func TestCloneResourceMetadataPreservesPreferredFormat(t *testing.T) {
	t.Parallel()

	cloned := CloneResourceMetadata(ResourceMetadata{PreferredFormat: "yaml"})
	if cloned.PreferredFormat != "yaml" {
		t.Fatalf("expected cloned preferredFormat yaml, got %#v", cloned)
	}
}

func TestMergeResourceMetadataOverlaysPreferredFormat(t *testing.T) {
	t.Parallel()

	merged := MergeResourceMetadata(
		ResourceMetadata{PreferredFormat: "json"},
		ResourceMetadata{PreferredFormat: "yaml"},
	)
	if merged.PreferredFormat != "yaml" {
		t.Fatalf("expected merged preferredFormat yaml, got %#v", merged)
	}
}

func TestCloneResourceMetadataClonesRequiredAttributes(t *testing.T) {
	t.Parallel()

	original := ResourceMetadata{RequiredAttributes: []string{"/name", "/realm"}}
	cloned := CloneResourceMetadata(original)

	if !reflect.DeepEqual(cloned.RequiredAttributes, original.RequiredAttributes) {
		t.Fatalf("expected cloned requiredAttributes %#v, got %#v", original.RequiredAttributes, cloned.RequiredAttributes)
	}
	cloned.RequiredAttributes[0] = "/displayName"
	if original.RequiredAttributes[0] != "/name" {
		t.Fatalf("expected original requiredAttributes to remain unchanged, got %#v", original.RequiredAttributes)
	}
}

func TestMergeResourceMetadataOverlaysRequiredAttributes(t *testing.T) {
	t.Parallel()

	merged := MergeResourceMetadata(
		ResourceMetadata{RequiredAttributes: []string{"/name"}},
		ResourceMetadata{RequiredAttributes: []string{"/realm", "/clientId"}},
	)
	if !reflect.DeepEqual(merged.RequiredAttributes, []string{"/realm", "/clientId"}) {
		t.Fatalf("expected merged requiredAttributes to use overlay, got %#v", merged.RequiredAttributes)
	}
}
