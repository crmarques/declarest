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

package orchestrator

import "testing"

func TestBuildDiffEntriesUsesResourcePathAndJSONPointerPaths(t *testing.T) {
	t.Parallel()

	items := buildDiffEntries(
		"/customers/acme",
		map[string]any{
			"a/b": map[string]any{
				"~name": "old",
			},
			"config": []any{
				map[string]any{"name": "old"},
			},
		},
		map[string]any{
			"a/b": map[string]any{
				"~name": "new",
			},
			"config": []any{
				map[string]any{"name": "new"},
			},
		},
	)

	if len(items) != 2 {
		t.Fatalf("expected two diff entries, got %#v", items)
	}

	expectedPointers := map[string]struct{}{
		"/a~1b/~0name":   {},
		"/config/0/name": {},
	}
	seenPointers := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ResourcePath != "/customers/acme" {
			t.Fatalf("expected resource path /customers/acme, got %#v", item.ResourcePath)
		}
		if _, ok := expectedPointers[item.Path]; !ok {
			t.Fatalf("unexpected pointer path %#v in %#v", item.Path, items)
		}
		seenPointers[item.Path] = struct{}{}
	}
	if len(seenPointers) != len(expectedPointers) {
		t.Fatalf("expected pointers %#v, got %#v", expectedPointers, seenPointers)
	}
}

func TestBuildDiffEntriesRootReplaceUsesEmptyPointer(t *testing.T) {
	t.Parallel()

	items := buildDiffEntries("/customers/acme", map[string]any{"id": "42"}, nil)
	if len(items) != 1 {
		t.Fatalf("expected one root replace entry, got %#v", items)
	}
	if items[0].ResourcePath != "/customers/acme" {
		t.Fatalf("expected resource path /customers/acme, got %#v", items[0].ResourcePath)
	}
	if items[0].Path != "" {
		t.Fatalf("expected empty pointer path for root replace, got %#v", items[0].Path)
	}
	if items[0].Operation != "replace" {
		t.Fatalf("expected replace operation, got %#v", items[0].Operation)
	}
}
