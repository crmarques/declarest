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

	t.Run("non_resolver_metadata_disables_fallback_for_empty_list", func(t *testing.T) {
		t.Parallel()

		if ShouldUseMetadataCollectionFallback(context.Background(), nil, "/a/b", nil) {
			t.Fatal("expected non-resolver metadata to disable fallback")
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

	t.Run("matching_metadata_child_enables_fallback_for_empty_list", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{children: []string{"user-registry", "clients"}}

		ok := ShouldUseMetadataCollectionFallback(
			context.Background(),
			resolver,
			"/admin/realms/acme/user-registry",
			nil,
		)
		if !ok {
			t.Fatal("expected metadata child match to enable fallback for empty collections")
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

	t.Run("wildcard_item_child_does_not_enable_collection_fallback", func(t *testing.T) {
		t.Parallel()

		resolver := &fakeMetadataCollectionChildren{wildcard: true}
		items := []resource.Resource{{LogicalPath: "/projects/asdfads/platform"}}

		ok := ShouldUseMetadataCollectionFallback(
			context.Background(),
			resolver,
			"/projects/asdfads",
			items,
		)
		if ok {
			t.Fatal("expected wildcard item metadata to keep child resource path as not found")
		}
		if resolver.wildcardLastPath != "" {
			t.Fatalf("expected no wildcard lookup for collection fallback, got %q", resolver.wildcardLastPath)
		}
	})
}
