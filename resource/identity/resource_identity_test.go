package identity

import (
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

	value, ok := LookupScalarAttribute(payload, "spec.nested.slug")
	if !ok || value != "acme" {
		t.Fatalf("expected nested slug, got value=%q found=%t", value, ok)
	}

	_, ok = LookupScalarAttribute(payload, "spec.missing")
	if ok {
		t.Fatal("expected missing path lookup to be false")
	}
}

func TestResolveAliasAndRemoteIDDottedAttributes(t *testing.T) {
	t.Parallel()

	alias, remoteID, err := ResolveAliasAndRemoteID(
		"/customers/acme",
		metadata.ResourceMetadata{AliasFromAttribute: "spec.slug", IDFromAttribute: "spec.id"},
		map[string]any{"spec": map[string]any{"slug": "new-alias", "id": "42"}},
	)
	if err != nil {
		t.Fatalf("ResolveAliasAndRemoteID returned error: %v", err)
	}
	if alias != "new-alias" {
		t.Fatalf("expected alias new-alias, got %q", alias)
	}
	if remoteID != "42" {
		t.Fatalf("expected remote id 42, got %q", remoteID)
	}
}

func TestResolveAliasAndRemoteIDForListItemRequiresAlias(t *testing.T) {
	t.Parallel()

	_, _, err := ResolveAliasAndRemoteIDForListItem(
		map[string]any{"name": "x"},
		metadata.ResourceMetadata{AliasFromAttribute: "missing", IDFromAttribute: "missing2"},
	)
	if err == nil {
		t.Fatal("expected error when list item alias cannot be resolved")
	}
}
