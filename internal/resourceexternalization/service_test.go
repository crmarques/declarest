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

package resourceexternalization

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
)

func TestExtractExternalizesConfiguredStringAttributes(t *testing.T) {
	t.Parallel()

	result, err := Extract(
		map[string]any{
			"script": "echo hello",
			"spec": map[string]any{
				"template": map[string]any{
					"script": "echo nested",
				},
			},
		},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
			{
				Path:           "/spec/template/script",
				File:           "nested.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	wantPayload := map[string]any{
		"script": "{{include script.sh}}",
		"spec": map[string]any{
			"template": map[string]any{
				"script": "{{include nested.sh}}",
			},
		},
	}
	if !reflect.DeepEqual(wantPayload, result.Payload) {
		t.Fatalf("unexpected extracted payload %#v", result.Payload)
	}

	if len(result.Artifacts) != 2 {
		t.Fatalf("expected two artifacts, got %#v", result.Artifacts)
	}
	if string(result.Artifacts[0].Content) != "echo hello" {
		t.Fatalf("unexpected top-level artifact %#v", result.Artifacts[0])
	}
	if string(result.Artifacts[1].Content) != "echo nested" {
		t.Fatalf("unexpected nested artifact %#v", result.Artifacts[1])
	}
}

func TestExtractRejectsNonStringValues(t *testing.T) {
	t.Parallel()

	_, err := Extract(
		map[string]any{"script": map[string]any{"inline": true}},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be a string value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractExternalizesWildcardArrayAttributes(t *testing.T) {
	t.Parallel()

	result, err := Extract(
		map[string]any{
			"sequence": map[string]any{
				"commands": []any{
					map[string]any{"script": "echo first"},
					map[string]any{"exec": "echo inline"},
					map[string]any{"script": "echo third"},
				},
			},
		},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/sequence/commands/*/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	wantPayload := map[string]any{
		"sequence": map[string]any{
			"commands": []any{
				map[string]any{"script": "{{include script-0.sh}}"},
				map[string]any{"exec": "echo inline"},
				map[string]any{"script": "{{include script-2.sh}}"},
			},
		},
	}
	if !reflect.DeepEqual(wantPayload, result.Payload) {
		t.Fatalf("unexpected extracted payload %#v", result.Payload)
	}

	wantArtifacts := []struct {
		file    string
		content string
	}{
		{file: "script-0.sh", content: "echo first"},
		{file: "script-2.sh", content: "echo third"},
	}
	if len(result.Artifacts) != len(wantArtifacts) {
		t.Fatalf("expected %d artifacts, got %#v", len(wantArtifacts), result.Artifacts)
	}
	for idx, want := range wantArtifacts {
		if got := result.Artifacts[idx].File; got != want.file {
			t.Fatalf("expected artifact %d file %q, got %q", idx, want.file, got)
		}
		if got := string(result.Artifacts[idx].Content); got != want.content {
			t.Fatalf("expected artifact %d content %q, got %q", idx, want.content, got)
		}
	}
}

func TestExpandReplacesPlaceholderBackedAttributes(t *testing.T) {
	t.Parallel()

	result, err := Expand(
		context.Background(),
		fakeArtifactReader{
			files: map[string][]byte{
				"/customers/acme::script.sh": []byte("echo hello"),
			},
		},
		"/customers/acme",
		map[string]any{"script": "{{include script.sh}}"},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}

	want := map[string]any{"script": "echo hello"}
	if !reflect.DeepEqual(want, result) {
		t.Fatalf("unexpected expanded payload %#v", result)
	}
}

func TestExpandRejectsMissingReferencedFile(t *testing.T) {
	t.Parallel()

	_, err := Expand(
		context.Background(),
		fakeArtifactReader{},
		"/customers/acme",
		map[string]any{"script": "{{include script.sh}}"},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "references missing file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExpandReplacesWildcardArrayPlaceholders(t *testing.T) {
	t.Parallel()

	result, err := Expand(
		context.Background(),
		fakeArtifactReader{
			files: map[string][]byte{
				"/customers/acme::script-0.sh": []byte("echo first"),
				"/customers/acme::script-2.sh": []byte("echo third"),
			},
		},
		"/customers/acme",
		map[string]any{
			"sequence": map[string]any{
				"commands": []any{
					map[string]any{"script": "{{include script-0.sh}}"},
					map[string]any{"exec": "echo inline"},
					map[string]any{"script": "{{include script-2.sh}}"},
				},
			},
		},
		[]metadata.ResolvedExternalizedAttribute{
			{
				Path:           "/sequence/commands/*/script",
				File:           "script.sh",
				Template:       metadata.DefaultExternalizedAttributeTemplate,
				Mode:           metadata.ExternalizedAttributeModeText,
				SaveBehavior:   metadata.ExternalizedAttributeSaveBehaviorExternalize,
				RenderBehavior: metadata.ExternalizedAttributeRenderBehaviorInclude,
				Enabled:        true,
			},
		},
	)
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}

	want := map[string]any{
		"sequence": map[string]any{
			"commands": []any{
				map[string]any{"script": "echo first"},
				map[string]any{"exec": "echo inline"},
				map[string]any{"script": "echo third"},
			},
		},
	}
	if !reflect.DeepEqual(want, result) {
		t.Fatalf("unexpected expanded payload %#v", result)
	}
}

type fakeArtifactReader struct {
	files map[string][]byte
}

func (f fakeArtifactReader) ReadResourceArtifact(_ context.Context, logicalPath string, file string) ([]byte, error) {
	if f.files == nil {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource artifact %q not found for %q", file, logicalPath),
			nil,
		)
	}

	content, found := f.files[logicalPath+"::"+file]
	if !found {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource artifact %q not found for %q", file, logicalPath),
			nil,
		)
	}
	return append([]byte(nil), content...), nil
}
