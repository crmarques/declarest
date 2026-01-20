package resource

import (
	"reflect"
	"testing"
)

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

func TestResourceRecordCollectionPath(t *testing.T) {
	cases := []struct {
		name   string
		record ResourceRecord
		want   string
	}{
		{
			name: "metadata_override",
			record: ResourceRecord{
				Path: "/ignore/me",
				Meta: ResourceMetadata{
					ResourceInfo: &ResourceInfoMetadata{
						CollectionPath: " /items/ ",
					},
				},
			},
			want: "/items",
		},
		{
			name:   "collection_path",
			record: ResourceRecord{Path: "/items/"},
			want:   "/items",
		},
		{
			name:   "resource_path",
			record: ResourceRecord{Path: "/items/item-1"},
			want:   "/items",
		},
		{
			name:   "empty_path",
			record: ResourceRecord{Path: ""},
			want:   "/",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.record.CollectionPath()
			if got != tc.want {
				t.Fatalf("CollectionPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResourceRecordResolveOperationPath(t *testing.T) {
	res, err := NewResource(map[string]any{
		"id":   "123",
		"name": "alias-1",
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	record := ResourceRecord{
		Path: "/items/alias-1",
		Data: res,
		Meta: ResourceMetadata{
			ResourceInfo: &ResourceInfoMetadata{
				IDFromAttribute:    "id",
				AliasFromAttribute: "name",
				CollectionPath:     "/items",
			},
		},
	}

	cases := []struct {
		name     string
		template string
		want     string
		wantErr  bool
	}{
		{name: "dot", template: ".", want: "/items"},
		{name: "relative_collection", template: "./mappers", want: "/items/mappers"},
		{name: "absolute_template", template: "/custom/{{.alias}}", want: "/custom/alias-1"},
		{name: "relative_path", template: "custom/{{.id}}", want: "custom/123"},
		{name: "invalid_template", template: "{{", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := &OperationMetadata{
				URL: &OperationURLMetadata{
					Path: tc.template,
				},
			}
			got, err := record.ResolveOperationPath(record.Path, op, false)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for template %q", tc.template)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveOperationPath: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ResolveOperationPath(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}

func TestResourceRecordQueryFor(t *testing.T) {
	record := ResourceRecord{}
	if got := record.QueryFor(nil); len(got) != 0 {
		t.Fatalf("expected empty query map, got %#v", got)
	}

	op := &OperationMetadata{
		URL: &OperationURLMetadata{
			QueryStrings: []string{
				"a=1",
				"a=2",
				"b",
				" =skip",
				"c= 3 ",
			},
		},
	}
	got := record.QueryFor(op)
	want := map[string][]string{
		"a": []string{"1", "2"},
		"b": []string{""},
		"c": []string{"3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("QueryFor() = %#v, want %#v", got, want)
	}
}
