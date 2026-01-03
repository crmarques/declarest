package resource

import "testing"

func TestResourceRecordAliasPath_UsesLocalBasePathForItems(t *testing.T) {
	record := ResourceRecord{
		Path: "/admin/realms/publico/user-registry/ldap-test",
		Meta: ResourceMetadata{
			ResourceInfo: &ResourceInfoMetadata{
				AliasFromAttribute: "name",
				CollectionPath:     "/admin/realms/publico/components",
			},
		},
	}

	res, err := NewResource(map[string]any{"name": "ldap-test"})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	got := record.AliasPath(res)
	want := "/admin/realms/publico/user-registry/ldap-test"
	if got != want {
		t.Fatalf("AliasPath() = %q, want %q", got, want)
	}
}

func TestResourceRecordAliasPath_UsesLocalBasePathForCollections(t *testing.T) {
	record := ResourceRecord{
		Path: "/admin/realms/publico/user-registry/ldap-test/mappers/",
		Meta: ResourceMetadata{
			ResourceInfo: &ResourceInfoMetadata{
				AliasFromAttribute: "name",
				CollectionPath:     "/admin/realms/publico/components",
			},
		},
	}

	res, err := NewResource(map[string]any{"name": "email"})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	got := record.AliasPath(res)
	want := "/admin/realms/publico/user-registry/ldap-test/mappers/email"
	if got != want {
		t.Fatalf("AliasPath() = %q, want %q", got, want)
	}
}

func TestResourceRecordHeadersForRendersTemplatesAndDefaults(t *testing.T) {
	res, err := NewResource(map[string]any{
		"id":   "123",
		"name": "foo",
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	record := ResourceRecord{
		Path: "/items/foo",
		Data: res,
		Meta: ResourceMetadata{
			ResourceInfo: &ResourceInfoMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
			},
			OperationInfo: &OperationInfoMetadata{
				GetResource: &OperationMetadata{
					HTTPMethod: "POST",
					HTTPHeaders: HeaderList{
						"X-Item: {{.alias}}",
						"X-Id: {{.id}}",
					},
				},
			},
		},
	}

	headers := record.HeadersFor(record.Meta.OperationInfo.GetResource, "/items/foo", false)
	if got := headers["X-Item"]; len(got) != 1 || got[0] != "foo" {
		t.Fatalf("expected X-Item header to be foo, got %#v", headers)
	}
	if got := headers["X-Id"]; len(got) != 1 || got[0] != "123" {
		t.Fatalf("expected X-Id header to be 123, got %#v", headers)
	}
	if got := headers["Accept"]; len(got) == 0 || got[0] != "application/json" {
		t.Fatalf("expected Accept header to be application/json, got %#v", headers)
	}
	if got := headers["Content-Type"]; len(got) == 0 || got[0] != "application/json" {
		t.Fatalf("expected Content-Type header to be application/json, got %#v", headers)
	}
}

func TestResourceRecordApplyPayloadTransforms(t *testing.T) {
	res, err := NewResource(map[string]any{
		"id":     "1",
		"name":   "keep",
		"secret": "drop",
		"nested": map[string]any{
			"keep": "yes",
			"drop": "no",
		},
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	payload := &OperationPayloadConfig{
		FilterAttributes:   []string{"id", "nested.keep", "nested.drop"},
		SuppressAttributes: []string{"secret", "nested.drop"},
		JQExpression:       ".nested",
	}

	record := ResourceRecord{}
	got, err := record.ApplyPayload(res, payload)
	if err != nil {
		t.Fatalf("ApplyPayload: %v", err)
	}

	obj, ok := got.AsObject()
	if !ok {
		t.Fatalf("expected object payload, got %#v", got.V)
	}
	if len(obj) != 1 || obj["keep"] != "yes" {
		t.Fatalf("unexpected payload: %#v", obj)
	}
}

func TestResourceRecordAliasPathSanitizesSegments(t *testing.T) {
	record := ResourceRecord{
		Path: "/items/foo",
		Meta: ResourceMetadata{
			ResourceInfo: &ResourceInfoMetadata{
				AliasFromAttribute: "name",
			},
		},
	}

	res, err := NewResource(map[string]any{"name": "a/b\\c"})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	got := record.AliasPath(res)
	want := "/items/a-b-c"
	if got != want {
		t.Fatalf("AliasPath() = %q, want %q", got, want)
	}
}
