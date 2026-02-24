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
	if scope["collectionPath"] != "/customers" {
		t.Fatalf("unexpected collectionPath: %#v", scope["collectionPath"])
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
					Path: "/api/projects/{{.project}}/widgets/{{.id}}",
				},
				string(metadata.OperationDelete): {
					Path: "/api/projects/{{.project}}/widgets/{{.id}}",
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
					Path: "/customers/{{.id}}",
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
			CollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path: "./{{.id}}",
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
			CollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					JQ: `[ .[] | select(.parentId == (resource("/admin/realms/{{.realm}}/user-registry/{{.provider}}/") | .id)) ]`,
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
		Metadata: metadata.ResourceMetadata{
			CollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path: "./{{.id}}",
				},
			},
		},
		Payload: map[string]any{"id": "123"},
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

func TestBuildResourceScopeInjectsJQResourceDerivedPathFields(t *testing.T) {
	t.Parallel()

	scope, err := BuildResourceScope(resource.Resource{
		LogicalPath:    "/admin/realms/aaa/user-registry/bbb/mappers/ccc",
		CollectionPath: "/admin/realms/aaa/user-registry/bbb/mappers",
		LocalAlias:     "ccc",
		RemoteID:       "mapper-id",
		Metadata: metadata.ResourceMetadata{
			CollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					JQ: `[ .[] | select(.parentId == (resource("/admin/realms/{{.realm}}/user-registry/{{.provider}}/") | .id)) ]`,
				},
			},
		},
		Payload: map[string]any{"id": "mapper-id"},
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
