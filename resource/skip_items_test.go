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

package resource

import (
	"reflect"
	"testing"
)

func TestShouldSkipCollectionItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		collection  string
		item        Resource
		excluded    []string
		wantSkipped bool
	}{
		{
			name:        "matches_local_alias",
			collection:  "/admin/realms",
			item:        Resource{LogicalPath: "/admin/realms/master", LocalAlias: "master", RemoteID: "realm-master"},
			excluded:    []string{"master"},
			wantSkipped: true,
		},
		{
			name:        "matches_remote_id",
			collection:  "/admin/realms",
			item:        Resource{LogicalPath: "/admin/realms/publico", LocalAlias: "publico", RemoteID: "realm-1"},
			excluded:    []string{"realm-1"},
			wantSkipped: true,
		},
		{
			name:        "matches_child_segment",
			collection:  "/admin/realms",
			item:        Resource{LogicalPath: "/admin/realms/tenant-a"},
			excluded:    []string{"tenant-a"},
			wantSkipped: true,
		},
		{
			name:        "matches_full_logical_path",
			collection:  "/admin/realms",
			item:        Resource{LogicalPath: "/admin/realms/tenant-b"},
			excluded:    []string{"/admin/realms/tenant-b"},
			wantSkipped: true,
		},
		{
			name:        "keeps_non_matching_item",
			collection:  "/admin/realms",
			item:        Resource{LogicalPath: "/admin/realms/tenant-c", LocalAlias: "tenant-c", RemoteID: "realm-c"},
			excluded:    []string{"master", "realm-1"},
			wantSkipped: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ShouldSkipCollectionItem(tt.collection, tt.item, tt.excluded); got != tt.wantSkipped {
				t.Fatalf("ShouldSkipCollectionItem() = %v, want %v", got, tt.wantSkipped)
			}
		})
	}
}

func TestFilterCollectionItems(t *testing.T) {
	t.Parallel()

	items := []Resource{
		{LogicalPath: "/customers/acme", LocalAlias: "acme", RemoteID: "42"},
		{LogicalPath: "/customers/beta", LocalAlias: "beta", RemoteID: "84"},
	}

	got := FilterCollectionItems("/customers", items, []string{"beta", "missing"})
	if !reflect.DeepEqual(got, []Resource{{LogicalPath: "/customers/acme", LocalAlias: "acme", RemoteID: "42"}}) {
		t.Fatalf("unexpected filtered items: %#v", got)
	}
}
