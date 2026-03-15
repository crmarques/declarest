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

func TestCloneResourceMetadataPreservesFormat(t *testing.T) {
	t.Parallel()

	cloned := CloneResourceMetadata(ResourceMetadata{Format: "yaml"})
	if cloned.Format != "yaml" {
		t.Fatalf("expected cloned format yaml, got %#v", cloned)
	}
}

func TestMergeResourceMetadataOverlaysFormat(t *testing.T) {
	t.Parallel()

	merged := MergeResourceMetadata(
		ResourceMetadata{Format: "json"},
		ResourceMetadata{Format: "yaml"},
	)
	if merged.Format != "yaml" {
		t.Fatalf("expected merged format yaml, got %#v", merged)
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

func TestCloneResourceMetadataClonesSelector(t *testing.T) {
	t.Parallel()

	original := ResourceMetadata{
		Selector: &SelectorSpec{Descendants: boolPointer(true)},
	}
	cloned := CloneResourceMetadata(original)

	if !cloned.Selector.AllowsDescendants() {
		t.Fatalf("expected cloned selector descendants=true, got %#v", cloned.Selector)
	}
	if cloned.Selector == original.Selector || cloned.Selector.Descendants == original.Selector.Descendants {
		t.Fatal("expected cloned selector pointers to be independent")
	}
}

func TestMergeResourceMetadataDoesNotMergeSelector(t *testing.T) {
	t.Parallel()

	merged := MergeResourceMetadata(
		ResourceMetadata{Selector: &SelectorSpec{Descendants: boolPointer(true)}},
		ResourceMetadata{Selector: &SelectorSpec{Descendants: boolPointer(true)}},
	)
	if merged.Selector != nil {
		t.Fatalf("expected selector to stay out of merged runtime metadata, got %#v", merged.Selector)
	}
}

func TestHasResourceMetadataDirectivesRecognizesNonIdentityOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata ResourceMetadata
	}{
		{
			name:     "required_attributes",
			metadata: ResourceMetadata{RequiredAttributes: []string{}},
		},
		{
			name:     "format",
			metadata: ResourceMetadata{Format: "text"},
		},
		{
			name:     "mixed_format",
			metadata: ResourceMetadata{Format: "any"},
		},
		{
			name:     "externalized_attributes",
			metadata: ResourceMetadata{ExternalizedAttributes: []ExternalizedAttribute{}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !HasResourceMetadataDirectives(tt.metadata) {
				t.Fatalf("expected metadata directives to be detected for %#v", tt.metadata)
			}
		})
	}
}

func TestHasResourceMetadataDirectivesIgnoresSelectorOnlyMetadata(t *testing.T) {
	t.Parallel()

	if HasResourceMetadataDirectives(ResourceMetadata{
		Selector: &SelectorSpec{Descendants: boolPointer(true)},
	}) {
		t.Fatal("expected selector-only metadata to not count as runtime directives")
	}
}

func TestCloneResourceMetadataDeepCopiesBody(t *testing.T) {
	t.Parallel()

	originalBody := map[string]any{"key": "original"}
	original := ResourceMetadata{
		Operations: map[string]OperationSpec{
			"create": {Body: originalBody},
		},
	}
	cloned := CloneResourceMetadata(original)

	clonedBody := cloned.Operations["create"].Body.(map[string]any)
	clonedBody["key"] = "mutated"

	if original.Operations["create"].Body.(map[string]any)["key"] != "original" {
		t.Fatal("expected original Body to remain unchanged after mutating clone")
	}
}

func TestMergeOperationSpecDeepCopiesBody(t *testing.T) {
	t.Parallel()

	baseBody := map[string]any{"field": "base"}
	base := OperationSpec{Body: baseBody}
	overlay := OperationSpec{}

	merged := MergeOperationSpec(base, overlay)

	mergedBody := merged.Body.(map[string]any)
	mergedBody["field"] = "mutated"

	if baseBody["field"] != "base" {
		t.Fatal("expected base Body to remain unchanged after mutating merged result")
	}
}

func TestMergeOperationSpecEmptyMapClearsInheritedHeaders(t *testing.T) {
	t.Parallel()

	base := OperationSpec{Headers: map[string]string{"X-Custom": "value"}}
	overlay := OperationSpec{Headers: map[string]string{}}

	merged := MergeOperationSpec(base, overlay)
	if len(merged.Headers) != 0 {
		t.Fatalf("expected empty overlay headers to clear inherited headers, got %v", merged.Headers)
	}
}

func TestMergeOperationSpecNilMapPreservesInheritedHeaders(t *testing.T) {
	t.Parallel()

	base := OperationSpec{Headers: map[string]string{"X-Custom": "value"}}
	overlay := OperationSpec{Headers: nil}

	merged := MergeOperationSpec(base, overlay)
	if merged.Headers["X-Custom"] != "value" {
		t.Fatalf("expected nil overlay headers to preserve inherited headers, got %v", merged.Headers)
	}
}

func TestMergeResourceMetadataStringFieldCannotBeClearedToEmpty(t *testing.T) {
	t.Parallel()

	base := ResourceMetadata{ID: "{{/id}}"}
	overlay := ResourceMetadata{ID: ""}

	merged := MergeResourceMetadata(base, overlay)
	if merged.ID != "{{/id}}" {
		t.Fatalf("expected empty overlay ID to preserve base ID, got %q", merged.ID)
	}
}

func TestMergeResourceMetadataSecretExplicitFalseOverridesTrue(t *testing.T) {
	t.Parallel()

	base := ResourceMetadata{Secret: boolPointer(true)}
	overlay := ResourceMetadata{Secret: boolPointer(false)}

	merged := MergeResourceMetadata(base, overlay)
	if merged.Secret == nil || *merged.Secret != false {
		t.Fatalf("expected explicit false to override true, got %v", merged.Secret)
	}
}
