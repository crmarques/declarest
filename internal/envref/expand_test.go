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

package envref

import "testing"

func TestExpandExactEnvPlaceholdersClonesAndResolvesNestedValues(t *testing.T) {
	t.Setenv("TEST_ENVREF_NAME", "declarest")
	t.Setenv("TEST_ENVREF_TOKEN", "secret-token")

	type nested struct {
		Value string
	}
	type sample struct {
		Name    string
		Partial string
		Nested  nested
		Items   []string
		Labels  map[string]string
		Any     any
	}

	original := sample{
		Name:    "${TEST_ENVREF_NAME}",
		Partial: "prefix-${TEST_ENVREF_NAME}",
		Nested:  nested{Value: "${TEST_ENVREF_TOKEN}"},
		Items:   []string{"${TEST_ENVREF_NAME}", "keep"},
		Labels:  map[string]string{"token": "${TEST_ENVREF_TOKEN}"},
		Any: map[string]any{
			"name": "${TEST_ENVREF_NAME}",
		},
	}

	expanded := ExpandExactEnvPlaceholders(original)

	if expanded.Name != "declarest" {
		t.Fatalf("expected Name to resolve, got %q", expanded.Name)
	}
	if expanded.Partial != "prefix-${TEST_ENVREF_NAME}" {
		t.Fatalf("expected partial placeholder to remain unchanged, got %q", expanded.Partial)
	}
	if expanded.Nested.Value != "secret-token" {
		t.Fatalf("expected nested value to resolve, got %q", expanded.Nested.Value)
	}
	if expanded.Items[0] != "declarest" || expanded.Items[1] != "keep" {
		t.Fatalf("unexpected expanded slice: %#v", expanded.Items)
	}
	if expanded.Labels["token"] != "secret-token" {
		t.Fatalf("expected map value to resolve, got %#v", expanded.Labels)
	}

	anyMap, ok := expanded.Any.(map[string]any)
	if !ok {
		t.Fatalf("expected interface map, got %#v", expanded.Any)
	}
	if anyMap["name"] != "declarest" {
		t.Fatalf("expected interface value to resolve, got %#v", anyMap)
	}

	if original.Name != "${TEST_ENVREF_NAME}" || original.Nested.Value != "${TEST_ENVREF_TOKEN}" {
		t.Fatalf("expected original value to remain unchanged, got %#v", original)
	}
}

func TestExpandExactEnvPlaceholdersUsesEmptyStringForMissingValues(t *testing.T) {
	t.Parallel()

	type sample struct {
		Value string
	}

	expanded := ExpandExactEnvPlaceholders(sample{Value: "${TEST_ENVREF_MISSING}"})
	if expanded.Value != "" {
		t.Fatalf("expected missing env placeholder to resolve to empty string, got %q", expanded.Value)
	}
}

func TestExpandExactEnvPlaceholdersInPlaceResolvesPointers(t *testing.T) {
	t.Setenv("TEST_ENVREF_URL", "https://example.com")

	type nested struct {
		URL string
	}
	type sample struct {
		Nested *nested
	}

	value := sample{Nested: &nested{URL: "${TEST_ENVREF_URL}"}}
	ExpandExactEnvPlaceholdersInPlace(&value)

	if value.Nested == nil || value.Nested.URL != "https://example.com" {
		t.Fatalf("expected in-place expansion to resolve pointer fields, got %#v", value)
	}

	ExpandExactEnvPlaceholdersInPlace(nil)
	ExpandExactEnvPlaceholdersInPlace(value)
}
