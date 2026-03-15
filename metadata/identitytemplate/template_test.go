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

package identitytemplate

import (
	"strings"
	"testing"
)

func TestCompileValidTemplates(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"{{/name}}",
		"{{name}}",
		"/name",
		"{{/name}} - {{/version}}",
		"{{to_uppercase /name}}",
		"{{default name \"/fallback\"}}",
		"{{substring /name 0 3}}",
		"{{default /missing \"/fallback\"}}",
	} {
		if _, err := Compile(raw); err != nil {
			t.Fatalf("Compile(%q) returned error: %v", raw, err)
		}
	}
}

func TestCompileRejectsInvalidTemplates(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"{{",
		"{{ }}",
		"{{unknown /name}}",
		"{{substring /name}}",
		"{{/name}",
		"{{\"unterminated}}",
	} {
		if _, err := Compile(raw); err == nil {
			t.Fatalf("Compile(%q) expected error", raw)
		}
	}
}

func TestRenderTemplateWithPointers(t *testing.T) {
	t.Parallel()

	rendered, err := Render("{{/name}} - {{/version}}", map[string]any{
		"name":    "widget",
		"version": "v2",
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if rendered != "widget - v2" {
		t.Fatalf("expected rendered template, got %q", rendered)
	}
}

func TestRenderPointerShorthand(t *testing.T) {
	t.Parallel()

	rendered, err := Render("/name", map[string]any{"name": "widget"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if rendered != "widget" {
		t.Fatalf("expected pointer shorthand to resolve widget, got %q", rendered)
	}
}

func TestRenderSingleLevelPointerTemplateShorthand(t *testing.T) {
	t.Parallel()

	rendered, err := Render("{{name}}", map[string]any{"name": "widget"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if rendered != "widget" {
		t.Fatalf("expected single-level shorthand to resolve widget, got %q", rendered)
	}
}

func TestRenderTemplateHelpers(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"name":          "  Widget  ",
		"metadata":      map[string]any{"externalId": "ext-1"},
		"numericSuffix": 42,
	}

	tests := []struct {
		raw  string
		want string
	}{
		{raw: "{{uppercase /name}}", want: "WIDGET"},
		{raw: "{{uppercase name}}", want: "WIDGET"},
		{raw: "{{lowercase /name}}", want: "widget"},
		{raw: "{{trim /name}}", want: "Widget"},
		{raw: "{{substring /name 0 3}}", want: "Wid"},
		{raw: "{{default /missing /metadata/externalId}}", want: "ext-1"},
		{raw: "{{default missing /metadata/externalId}}", want: "ext-1"},
		{raw: "{{default /missing /numericSuffix}}", want: "42"},
	}

	for _, test := range tests {
		rendered, err := Render(test.raw, payload)
		if err != nil {
			t.Fatalf("Render(%q) returned error: %v", test.raw, err)
		}
		if rendered != test.want {
			t.Fatalf("Render(%q) = %q, want %q", test.raw, rendered, test.want)
		}
	}
}

func TestRenderTemplateReturnsClearMissingValueError(t *testing.T) {
	t.Parallel()

	_, err := Render("{{/missing}}", map[string]any{"name": "widget"})
	if err == nil {
		t.Fatal("expected missing-value error")
	}
	if !strings.Contains(err.Error(), `"/missing"`) {
		t.Fatalf("expected missing pointer in error, got %v", err)
	}
}

func TestExtractPointers(t *testing.T) {
	t.Parallel()

	pointers, err := ExtractPointers("{{default /name /metadata/externalId}} - {{substring /version 0 1}}")
	if err != nil {
		t.Fatalf("ExtractPointers returned error: %v", err)
	}

	want := []string{"/name", "/metadata/externalId", "/version"}
	if len(pointers) != len(want) {
		t.Fatalf("expected %d pointers, got %#v", len(want), pointers)
	}
	for idx := range want {
		if pointers[idx] != want[idx] {
			t.Fatalf("unexpected pointers %#v", pointers)
		}
	}
}

func TestSimplePointerDetection(t *testing.T) {
	t.Parallel()

	pointer, ok, err := SimplePointer("{{/name}}")
	if err != nil {
		t.Fatalf("SimplePointer returned error: %v", err)
	}
	if !ok || pointer != "/name" {
		t.Fatalf("expected simple pointer /name, got ok=%v pointer=%q", ok, pointer)
	}

	pointer, ok, err = SimplePointer("/name")
	if err != nil {
		t.Fatalf("SimplePointer returned error: %v", err)
	}
	if !ok || pointer != "/name" {
		t.Fatalf("expected shorthand simple pointer /name, got ok=%v pointer=%q", ok, pointer)
	}

	pointer, ok, err = SimplePointer("{{name}}")
	if err != nil {
		t.Fatalf("SimplePointer returned error: %v", err)
	}
	if !ok || pointer != "/name" {
		t.Fatalf("expected bare shorthand simple pointer /name, got ok=%v pointer=%q", ok, pointer)
	}

	pointer, ok, err = SimplePointer("{{/name}}-{{/version}}")
	if err != nil {
		t.Fatalf("SimplePointer returned error: %v", err)
	}
	if ok || pointer != "" {
		t.Fatalf("expected non-simple template, got ok=%v pointer=%q", ok, pointer)
	}
}
