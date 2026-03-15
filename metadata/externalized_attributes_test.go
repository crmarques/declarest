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
	"strings"
	"testing"
)

func TestResolveExternalizedAttributesAppliesDefaults(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveExternalizedAttributes(ResourceMetadata{
		ExternalizedAttributes: []ExternalizedAttribute{
			{
				Path: "/script",
				File: "script.sh",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveExternalizedAttributes returned error: %v", err)
	}

	want := []ResolvedExternalizedAttribute{
		{
			Path:           "/script",
			File:           "script.sh",
			Template:       DefaultExternalizedAttributeTemplate,
			Mode:           ExternalizedAttributeModeText,
			SaveBehavior:   ExternalizedAttributeSaveBehaviorExternalize,
			RenderBehavior: ExternalizedAttributeRenderBehaviorInclude,
			Enabled:        true,
		},
	}
	if !reflect.DeepEqual(want, resolved) {
		t.Fatalf("unexpected resolved externalized attributes: %#v", resolved)
	}
}

func TestResolveExternalizedAttributesIgnoresDisabledEntries(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveExternalizedAttributes(ResourceMetadata{
		ExternalizedAttributes: []ExternalizedAttribute{
			{
				Path:    "/script",
				File:    "ignored.sh",
				Enabled: boolPointer(false),
			},
			{
				Path: "/spec/template/script",
				File: "script.sh",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveExternalizedAttributes returned error: %v", err)
	}

	if len(resolved) != 1 {
		t.Fatalf("expected one enabled entry, got %#v", resolved)
	}
	if resolved[0].Path != "/spec/template/script" {
		t.Fatalf("unexpected nested path %#v", resolved[0].Path)
	}
}

func TestResolveExternalizedAttributesValidatesDuplicatesAndPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		metadata ResourceMetadata
		wantErr  string
	}{
		{
			name: "duplicate_path",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: "/script", File: "script-1.sh"},
					{Path: "/script", File: "script-2.sh"},
				},
			},
			wantErr: "duplicates",
		},
		{
			name: "duplicate_file",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: "/script", File: "script.sh"},
					{Path: "/config", File: "script.sh"},
				},
			},
			wantErr: "duplicates",
		},
		{
			name: "empty_path",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{File: "script.sh"},
				},
			},
			wantErr: "path must not be empty",
		},
		{
			name: "empty_file",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: "/script"},
				},
			},
			wantErr: "file must not be empty",
		},
		{
			name: "path_traversal",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: "/script", File: "../script.sh"},
				},
			},
			wantErr: "must stay within the resource directory",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ResolveExternalizedAttributes(tc.metadata)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}
