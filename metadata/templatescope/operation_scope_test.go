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

package templatescope

import (
	"testing"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestBuildOperationScopePreservesPayloadBinding(t *testing.T) {
	t.Parallel()

	scope, err := BuildOperationScope(
		"/customers/acme",
		"/customers",
		"acme",
		"42",
		map[string]any{
			"payload": "data",
			"tenant":  "north",
		},
	)
	if err != nil {
		t.Fatalf("BuildOperationScope returned error: %v", err)
	}

	if scope["logicalPath"] != "/customers/acme" {
		t.Fatalf("unexpected logicalPath: %#v", scope["logicalPath"])
	}
	if scope["logicalCollectionPath"] != "/customers" {
		t.Fatalf("unexpected logicalCollectionPath: %#v", scope["logicalCollectionPath"])
	}
	if scope["remoteCollectionPath"] != "/customers" {
		t.Fatalf("unexpected remoteCollectionPath: %#v", scope["remoteCollectionPath"])
	}
	if scope["alias"] != "acme" {
		t.Fatalf("unexpected alias: %#v", scope["alias"])
	}
	if scope["remoteID"] != "42" {
		t.Fatalf("unexpected remoteID: %#v", scope["remoteID"])
	}
	payloadMap, ok := scope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload to be map, got %T", scope["payload"])
	}
	if payloadMap["tenant"] != "north" {
		t.Fatalf("unexpected payload map: %#v", payloadMap)
	}

	valueMap, ok := scope["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected value to be map, got %T", scope["value"])
	}

	payloadMap["tenant"] = "south"
	if valueMap["tenant"] != "south" {
		t.Fatal("expected payload and value to reference the same map scope")
	}
}

func TestDerivePathTemplateFields(t *testing.T) {
	t.Parallel()

	fields := DerivePathTemplateFields(
		"/api/projects/platform/widgets/beta",
		metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path: "/api/projects/{{/project}}/widgets/{{/id}}",
				},
				string(metadata.OperationDelete): {
					Path: "/api/projects/{{/project}}/widgets/{{/id}}",
				},
			},
		},
	)

	if fields["project"] != "platform" {
		t.Fatalf("expected project field to be derived, got %#v", fields["project"])
	}
	if fields["id"] != "beta" {
		t.Fatalf("expected id field to be derived, got %#v", fields["id"])
	}
}

func TestDerivePathTemplateFieldsSkipsMismatchedTemplate(t *testing.T) {
	t.Parallel()

	fields := DerivePathTemplateFields(
		"/api/projects/platform/widgets/beta",
		metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path: "/customers/{{/id}}",
				},
			},
		},
	)

	if len(fields) != 0 {
		t.Fatalf("expected no derived fields for mismatched template, got %#v", fields)
	}
}

func TestDerivePathTemplateFieldsFromCollectionTemplatePrefix(t *testing.T) {
	t.Parallel()

	fields := DerivePathTemplateFields(
		"/admin/realms/platform/user-registry",
		metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path: "./{{/id}}",
				},
			},
		},
	)

	if fields["realm"] != "platform" {
		t.Fatalf("expected realm field to be derived from collection template, got %#v", fields["realm"])
	}
}

func TestDerivePathTemplateFieldsFromListJQResourcePathTemplate(t *testing.T) {
	t.Parallel()

	fields := DerivePathTemplateFields(
		"/admin/realms/aaa/user-registry/bbb/mappers/ccc",
		metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Transforms: []metadata.TransformStep{
						{JQExpression: `[ .[] | select(.parentId == (resource("/admin/realms/{{/realm}}/user-registry/{{/provider}}/") | .id)) ]`},
					},
				},
			},
		},
	)

	if fields["realm"] != "aaa" {
		t.Fatalf("expected realm field to be derived from jq resource path, got %#v", fields["realm"])
	}
	if fields["provider"] != "bbb" {
		t.Fatalf("expected provider field to be derived from jq resource path, got %#v", fields["provider"])
	}
}

func TestBuildResourceScopeInjectsDerivedPathFields(t *testing.T) {
	t.Parallel()

	scope, err := BuildResourceScope(resource.Resource{
		LogicalPath:    "/admin/realms/platform/user-registry",
		CollectionPath: "/admin/realms/platform",
		LocalAlias:     "user-registry",
		RemoteID:       "123",
		Payload:        map[string]any{"id": "123"},
	}, metadata.ResourceMetadata{
		RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationGet): {
				Path: "./{{/id}}",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildResourceScope returned error: %v", err)
	}

	if scope["realm"] != "platform" {
		t.Fatalf("expected derived realm in scope, got %#v", scope["realm"])
	}
	payloadMap, ok := scope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map in scope, got %T", scope["payload"])
	}
	if payloadMap["realm"] != "platform" {
		t.Fatalf("expected derived realm in payload map, got %#v", payloadMap["realm"])
	}
}

func TestBuildResourceScopeInjectsPluralLogicalCollectionFields(t *testing.T) {
	t.Parallel()

	scope, err := BuildResourceScope(resource.Resource{
		LogicalPath:    "/projects/platform/secrets/db-password",
		CollectionPath: "/projects/platform/secrets",
		LocalAlias:     "db-password",
		RemoteID:       "db-password",
		Payload:        map[string]any{"name": "db-password"},
	}, metadata.ResourceMetadata{
		RemoteCollectionPath: "/storage/keys/project/{{/project}}",
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationGet): {
				Path: "./{{/id}}",
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildResourceScope returned error: %v", err)
	}

	if scope["project"] != "platform" {
		t.Fatalf("expected derived project in scope, got %#v", scope["project"])
	}
	payloadMap, ok := scope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map in scope, got %T", scope["payload"])
	}
	if payloadMap["project"] != "platform" {
		t.Fatalf("expected derived project in payload map, got %#v", payloadMap["project"])
	}
}

func TestBuildResourceScopeWithOptionsUsesDerivedCollectionPath(t *testing.T) {
	t.Parallel()

	scope, err := BuildResourceScopeWithOptions(resource.Resource{
		LogicalPath:    "/projects/platform/secrets/path/to/db-password",
		CollectionPath: "/projects/platform/secrets/path/to",
		LocalAlias:     "db-password",
		RemoteID:       "db-password",
		Payload:        map[string]any{"name": "db-password"},
	}, metadata.ResourceMetadata{
		RemoteCollectionPath: "/storage/keys/project/{{/project}}{{/descendantCollectionPath}}",
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationGet): {
				Path: "./{{/id}}",
			},
		},
	}, ResourceScopeOptions{
		DerivedCollectionPath: "/projects/platform/secrets",
	})
	if err != nil {
		t.Fatalf("BuildResourceScopeWithOptions returned error: %v", err)
	}

	if scope["project"] != "platform" {
		t.Fatalf("expected derived project in scope, got %#v", scope["project"])
	}
	if _, exists := scope["secret"]; exists {
		t.Fatalf("expected descendant path suffix not to become secret field, got %#v", scope["secret"])
	}
	payloadMap, ok := scope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map in scope, got %T", scope["payload"])
	}
	if _, exists := payloadMap["secret"]; exists {
		t.Fatalf("expected payload map to avoid bogus secret field, got %#v", payloadMap["secret"])
	}
}

func TestBuildResourceScopeInjectsJQResourceDerivedPathFields(t *testing.T) {
	t.Parallel()

	scope, err := BuildResourceScope(resource.Resource{
		LogicalPath:    "/admin/realms/aaa/user-registry/bbb/mappers/ccc",
		CollectionPath: "/admin/realms/aaa/user-registry/bbb/mappers",
		LocalAlias:     "ccc",
		RemoteID:       "mapper-id",
		Payload:        map[string]any{"id": "mapper-id"},
	}, metadata.ResourceMetadata{
		RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationList): {
				Transforms: []metadata.TransformStep{
					{JQExpression: `[ .[] | select(.parentId == (resource("/admin/realms/{{/realm}}/user-registry/{{/provider}}/") | .id)) ]`},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildResourceScope returned error: %v", err)
	}

	if scope["provider"] != "bbb" {
		t.Fatalf("expected derived provider in scope, got %#v", scope["provider"])
	}
	payloadMap, ok := scope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map in scope, got %T", scope["payload"])
	}
	if payloadMap["provider"] != "bbb" {
		t.Fatalf("expected derived provider in payload map, got %#v", payloadMap["provider"])
	}
}
