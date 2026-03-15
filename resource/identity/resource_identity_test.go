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

package identity

import (
	"strings"
	"testing"

	"github.com/crmarques/declarest/metadata"
)

func TestLookupScalarAttribute(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"id": "10",
		"spec": map[string]any{
			"nested": map[string]any{
				"slug": "acme",
			},
		},
	}

	value, ok := LookupScalarAttribute(payload, "/spec/nested/slug")
	if !ok || value != "acme" {
		t.Fatalf("expected nested slug, got value=%q found=%t", value, ok)
	}

	_, ok = LookupScalarAttribute(payload, "/spec/missing")
	if ok {
		t.Fatal("expected missing path lookup to be false")
	}
}

func TestResolveAliasAndRemoteIDTemplates(t *testing.T) {
	t.Parallel()

	alias, remoteID, err := ResolveAliasAndRemoteID(
		"/customers/acme",
		metadata.ResourceMetadata{
			Alias: "{{uppercase /spec/slug}}",
			ID:    "{{default /spec/externalId /spec/id}}",
		},
		map[string]any{"spec": map[string]any{"slug": "new-alias", "id": "42"}},
	)
	if err != nil {
		t.Fatalf("ResolveAliasAndRemoteID returned error: %v", err)
	}
	if alias != "NEW-ALIAS" {
		t.Fatalf("expected alias NEW-ALIAS, got %q", alias)
	}
	if remoteID != "42" {
		t.Fatalf("expected remote id 42, got %q", remoteID)
	}
}

func TestResolveAliasAndRemoteIDPointerShorthand(t *testing.T) {
	t.Parallel()

	alias, remoteID, err := ResolveAliasAndRemoteID(
		"/customers/acme",
		metadata.ResourceMetadata{
			Alias: "/name",
			ID:    "/id",
		},
		map[string]any{"name": "widget", "id": "42"},
	)
	if err != nil {
		t.Fatalf("ResolveAliasAndRemoteID returned error: %v", err)
	}
	if alias != "widget" {
		t.Fatalf("expected alias widget, got %q", alias)
	}
	if remoteID != "42" {
		t.Fatalf("expected remote id 42, got %q", remoteID)
	}
}

func TestResolveAliasAndRemoteIDDefaultsIdentityToIDPointer(t *testing.T) {
	t.Parallel()

	alias, remoteID, err := ResolveAliasAndRemoteID(
		"/customers/acme",
		metadata.ResourceMetadata{},
		map[string]any{"id": "42"},
	)
	if err != nil {
		t.Fatalf("ResolveAliasAndRemoteID returned error: %v", err)
	}
	if alias != "42" {
		t.Fatalf("expected alias 42, got %q", alias)
	}
	if remoteID != "42" {
		t.Fatalf("expected remote id 42, got %q", remoteID)
	}
}

func TestResolveAliasAndRemoteIDForListItemRequiresAlias(t *testing.T) {
	t.Parallel()

	_, _, err := ResolveAliasAndRemoteIDForListItem(
		map[string]any{"name": "x"},
		metadata.ResourceMetadata{Alias: "{{/missing}}", ID: "{{/missing2}}"},
	)
	if err == nil {
		t.Fatal("expected error when list item alias cannot be resolved")
	}
}

func TestResolveAliasAndRemoteIDForListItemDefaultsIdentityToIDPointer(t *testing.T) {
	t.Parallel()

	alias, remoteID, err := ResolveAliasAndRemoteIDForListItem(
		map[string]any{"id": "42"},
		metadata.ResourceMetadata{},
	)
	if err != nil {
		t.Fatalf("ResolveAliasAndRemoteIDForListItem returned error: %v", err)
	}
	if alias != "42" || remoteID != "42" {
		t.Fatalf("expected alias/remoteID 42, got alias=%q remoteID=%q", alias, remoteID)
	}
}

func TestResolveAliasAndRemoteIDRejectsSlashfulRenderedID(t *testing.T) {
	t.Parallel()

	_, _, err := ResolveAliasAndRemoteID(
		"/projects/platform/secrets/path/to/db-password",
		metadata.ResourceMetadata{
			ID: "{{/fullPath}}",
		},
		map[string]any{"fullPath": "path/to/db-password"},
	)
	if err == nil {
		t.Fatal("expected slashful resource.id render to be rejected")
	}
	if !strings.Contains(err.Error(), `resource.id rendered invalid logical path segment "path/to/db-password"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSimpleIdentityPointers(t *testing.T) {
	t.Parallel()

	md := metadata.ResourceMetadata{
		Alias: "{{/name}}",
		ID:    "{{/id}}-{{/version}}",
	}

	aliasPointer, ok, err := SimpleAliasPointer(md)
	if err != nil {
		t.Fatalf("SimpleAliasPointer returned error: %v", err)
	}
	if !ok || aliasPointer != "/name" {
		t.Fatalf("unexpected alias pointer ok=%v value=%q", ok, aliasPointer)
	}

	idPointer, ok, err := SimpleIDPointer(md)
	if err != nil {
		t.Fatalf("SimpleIDPointer returned error: %v", err)
	}
	if ok || idPointer != "" {
		t.Fatalf("expected complex id template to reject simple-pointer reverse mapping, got ok=%v value=%q", ok, idPointer)
	}
}
