package pathfallback

import (
	"context"
	"testing"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type fakeMetadataCollectionChildren struct {
	metadatadomain.MetadataService
	children         []string
	err              error
	lastPath         string
	wildcard         bool
	wildcardErr      error
	wildcardLastPath string
}

func (f *fakeMetadataCollectionChildren) ResolveCollectionChildren(_ context.Context, logicalPath string) ([]string, error) {
	f.lastPath = logicalPath
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.children...), nil
}

func (f *fakeMetadataCollectionChildren) HasCollectionWildcardChild(_ context.Context, logicalPath string) (bool, error) {
	f.wildcardLastPath = logicalPath
	if f.wildcardErr != nil {
		return false, f.wildcardErr
	}
	return f.wildcard, nil
}

func TestShouldUseMetadataCollectionFallback(t *testing.T) {
	t.Parallel()

	t.Run("empty_list_always_uses_fallback", func(t *testing.T) {
		t.Parallel()

		if !ShouldUseMetadataCollectionFallback(context.Background(), nil, "/a/b", nil) {
			t.Fatal("expected empty list to enable fallback")
		}
	})

	t.Run("non_resolver_metadata_disables_fallback_for_non_empty_list", func(t *testing.T) {
		t.Parallel()

		items := []resource.Resource{{LogicalPath: "/a/b"}}
		if ShouldUseMetadataCollectionFallback(context.Background(), nil, "/a/b", items) {
			t.Fatal("expected non-resolver metadata service to disable fallback")
		}
	})

	t.Run("invalid_or_root_paths_do_not_use_fallback", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{children: []string{"child"}}
		items := []resource.Resource{{LogicalPath: "/a/b"}}

		if ShouldUseMetadataCollectionFallback(context.Background(), resolver, "not-absolute", items) {
			t.Fatal("expected invalid path to disable fallback")
		}
		if ShouldUseMetadataCollectionFallback(context.Background(), resolver, "/", items) {
			t.Fatal("expected root path to disable fallback")
		}
	})

	t.Run("matching_metadata_child_enables_fallback", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{children: []string{"user-registry", "clients"}}
		items := []resource.Resource{{LogicalPath: "/admin/realms/acme"}}

		ok := ShouldUseMetadataCollectionFallback(
			context.Background(),
			resolver,
			"/admin/realms/acme/user-registry",
			items,
		)
		if !ok {
			t.Fatal("expected metadata child match to enable fallback")
		}
		if resolver.lastPath != "/admin/realms/acme" {
			t.Fatalf("expected parent path lookup, got %q", resolver.lastPath)
		}
	})

	t.Run("non_matching_metadata_child_disables_fallback", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{children: []string{"clients"}}
		items := []resource.Resource{{LogicalPath: "/admin/realms/acme"}}

		if ShouldUseMetadataCollectionFallback(context.Background(), resolver, "/admin/realms/acme/user-registry", items) {
			t.Fatal("expected missing child match to disable fallback")
		}
	})

	t.Run("wildcard_metadata_child_enables_fallback", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{wildcard: true}
		items := []resource.Resource{{LogicalPath: "/admin/realms/acme"}}

		ok := ShouldUseMetadataCollectionFallback(
			context.Background(),
			resolver,
			"/admin/realms/acme/authentication/flows/test/executions/Cookie",
			items,
		)
		if !ok {
			t.Fatal("expected wildcard metadata to enable fallback")
		}
		if resolver.wildcardLastPath != "/admin/realms/acme/authentication/flows/test/executions" {
			t.Fatalf("expected wildcard lookup on parent path, got %q", resolver.wildcardLastPath)
		}
	})
}
