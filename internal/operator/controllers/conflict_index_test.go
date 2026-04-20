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

package controllers

import "testing"

func TestConflictIndexNameLevelLookup(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	source := ConflictSource{
		CRDGeneratorNamespace: "ns1",
		CRDGeneratorName:      "realm",
		GeneratedKind:         "Realm",
		LogicalPath:           "/admin/realms/foo",
	}
	index.Register("ns1", "keycloak", "/admin/realms", "/admin/realms/foo", "", source)

	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "/admin/realms/foo", ConflictTierName); !found {
		t.Fatal("expected tier-1 hit for exact logical path")
	}
	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "/admin/realms/bar", ConflictTierName); found {
		t.Fatal("unexpected hit for unrelated path")
	}
}

func TestConflictIndexRemoteIDLookup(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	source := ConflictSource{
		CRDGeneratorNamespace: "ns1",
		CRDGeneratorName:      "realm",
		GeneratedKind:         "Realm",
		LogicalPath:           "/admin/realms/foo",
		RemoteID:              "abc-123",
	}
	index.Register("ns1", "keycloak", "/admin/realms", "/admin/realms/foo", "abc-123", source)

	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "abc-123", ConflictTierRemoteID); !found {
		t.Fatal("expected tier-2 hit for remote ID")
	}
	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "missing", ConflictTierRemoteID); found {
		t.Fatal("unexpected hit for unknown remote ID")
	}
}

func TestConflictIndexUnregister(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	source := ConflictSource{
		CRDGeneratorNamespace: "ns1",
		CRDGeneratorName:      "realm",
		GeneratedKind:         "Realm",
		LogicalPath:           "/admin/realms/foo",
	}
	index.Register("ns1", "keycloak", "/admin/realms", "/admin/realms/foo", "abc-123", source)

	index.Unregister("ns1", "realm", "")

	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "/admin/realms/foo", ConflictTierName); found {
		t.Fatal("expected entry to be removed by Unregister")
	}
	if _, found := index.Lookup("ns1", "keycloak", "/admin/realms", "abc-123", ConflictTierRemoteID); found {
		t.Fatal("expected tier-2 entry to be removed by Unregister")
	}
}

func TestConflictIndexDifferentManagedServicesDoNotCollide(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	index.Register("ns1", "keycloak-a", "/admin/realms", "/admin/realms/foo", "", ConflictSource{
		CRDGeneratorNamespace: "ns1",
		CRDGeneratorName:      "realm-a",
	})

	if _, found := index.Lookup("ns1", "keycloak-b", "/admin/realms", "/admin/realms/foo", ConflictTierName); found {
		t.Fatal("managed-service boundary leaked between entries")
	}
}

func TestConflictIndexNilReceiver(t *testing.T) {
	t.Parallel()

	var index *ConflictIndex
	// All methods MUST be safe on a nil receiver so callers can treat the
	// conflict index as optional.
	index.Register("ns", "svc", "/collection", "/collection/a", "", ConflictSource{})
	index.Unregister("ns", "svc", "")
	if _, found := index.Lookup("ns", "svc", "/collection", "/collection/a", ConflictTierName); found {
		t.Fatal("nil index must never return a hit")
	}
	if paths := index.CollectionPathsForGenerator("ns", "svc"); paths != nil {
		t.Fatal("nil index must return no collection paths")
	}
}
