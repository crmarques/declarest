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
				Path: []string{"script"},
				File: "script.sh",
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveExternalizedAttributes returned error: %v", err)
	}

	want := []ResolvedExternalizedAttribute{
		{
			Path:           []string{"script"},
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
				Path:    []string{"script"},
				File:    "ignored.sh",
				Enabled: boolPointer(false),
			},
			{
				Path: []string{"spec", "template", "script"},
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
	if !reflect.DeepEqual(resolved[0].Path, []string{"spec", "template", "script"}) {
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
					{Path: []string{"script"}, File: "script-1.sh"},
					{Path: []string{"script"}, File: "script-2.sh"},
				},
			},
			wantErr: "duplicates",
		},
		{
			name: "duplicate_file",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: []string{"script"}, File: "script.sh"},
					{Path: []string{"config"}, File: "script.sh"},
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
					{Path: []string{"script"}},
				},
			},
			wantErr: "file must not be empty",
		},
		{
			name: "path_traversal",
			metadata: ResourceMetadata{
				ExternalizedAttributes: []ExternalizedAttribute{
					{Path: []string{"script"}, File: "../script.sh"},
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
