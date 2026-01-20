package metadata

import (
	"reflect"
	"testing"
)

func TestSetMetadataAttributeCreatesNestedValue(t *testing.T) {
	meta := map[string]any{}

	changed, err := SetMetadataAttribute(meta, "resourceInfo.idFromAttribute", "id")
	if err != nil {
		t.Fatalf("SetMetadataAttribute: %v", err)
	}
	if !changed {
		t.Fatalf("expected change to be reported")
	}

	expected := map[string]any{
		"resourceInfo": map[string]any{
			"idFromAttribute": "id",
		},
	}
	if !reflect.DeepEqual(meta, expected) {
		t.Fatalf("unexpected metadata: %#v", meta)
	}
}

func TestSetMetadataAttributeAppendsArrayValues(t *testing.T) {
	meta := map[string]any{
		"resourceInfo": map[string]any{
			"secretInAttributes": []any{"token"},
		},
	}

	changed, err := SetMetadataAttribute(meta, "resourceInfo.secretInAttributes", "password")
	if err != nil {
		t.Fatalf("SetMetadataAttribute: %v", err)
	}
	if !changed {
		t.Fatalf("expected change to be reported")
	}

	resourceInfo := meta["resourceInfo"].(map[string]any)
	values := resourceInfo["secretInAttributes"].([]any)
	if !reflect.DeepEqual(values, []any{"token", "password"}) {
		t.Fatalf("unexpected array values: %#v", values)
	}

	changed, err = SetMetadataAttribute(meta, "resourceInfo.secretInAttributes", "password")
	if err != nil {
		t.Fatalf("SetMetadataAttribute: %v", err)
	}
	if changed {
		t.Fatalf("expected duplicate value to be ignored")
	}
}

func TestUnsetMetadataAttributeRemovesValueAndCleansUp(t *testing.T) {
	meta := map[string]any{
		"operationInfo": map[string]any{
			"listCollection": map[string]any{
				"url": map[string]any{
					"queryStrings": []any{"a=1"},
				},
			},
			"getResource": map[string]any{
				"httpMethod": "GET",
			},
		},
	}

	changed, err := UnsetMetadataAttribute(meta, "operationInfo.listCollection.url.queryStrings", "a=1")
	if err != nil {
		t.Fatalf("UnsetMetadataAttribute: %v", err)
	}
	if !changed {
		t.Fatalf("expected change to be reported")
	}

	expected := map[string]any{
		"operationInfo": map[string]any{
			"getResource": map[string]any{
				"httpMethod": "GET",
			},
		},
	}
	if !reflect.DeepEqual(meta, expected) {
		t.Fatalf("unexpected metadata after unset: %#v", meta)
	}
}

func TestDeleteMetadataAttributeRemovesAttribute(t *testing.T) {
	meta := map[string]any{
		"operationInfo": map[string]any{
			"getResource": map[string]any{
				"httpMethod": "GET",
			},
			"deleteResource": map[string]any{
				"httpMethod": "DELETE",
			},
		},
	}

	changed, err := DeleteMetadataAttribute(meta, "operationInfo.deleteResource")
	if err != nil {
		t.Fatalf("DeleteMetadataAttribute: %v", err)
	}
	if !changed {
		t.Fatalf("expected change to be reported")
	}

	expected := map[string]any{
		"operationInfo": map[string]any{
			"getResource": map[string]any{
				"httpMethod": "GET",
			},
		},
	}
	if !reflect.DeepEqual(meta, expected) {
		t.Fatalf("unexpected metadata after delete: %#v", meta)
	}
}
