package resource

import "testing"

func TestApplyCompareRulesTransforms(t *testing.T) {
	res, err := NewResource(map[string]any{
		"id":     "1",
		"name":   "foo",
		"status": "active",
		"meta": map[string]any{
			"updatedAt": "2024-01-01",
			"keep":      "yes",
		},
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	cmp := &CompareMetadata{
		IgnoreAttributes:   []string{"status"},
		SuppressAttributes: []string{"meta.updatedAt"},
		FilterAttributes:   []string{"id", "name", "meta.keep"},
	}

	got, err := ApplyCompareRules(res, cmp)
	if err != nil {
		t.Fatalf("ApplyCompareRules: %v", err)
	}

	obj, ok := got.AsObject()
	if !ok {
		t.Fatalf("expected object payload, got %#v", got.V)
	}
	if _, ok := obj["status"]; ok {
		t.Fatalf("expected status to be removed, got %#v", obj)
	}
	meta, ok := obj["meta"].(map[string]any)
	if !ok || meta["keep"] != "yes" || meta["updatedAt"] != nil {
		t.Fatalf("unexpected meta payload: %#v", obj)
	}
}

func TestApplyCompareRulesAppliesJQ(t *testing.T) {
	res, err := NewResource(map[string]any{
		"id":   "1",
		"name": "foo",
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	cmp := &CompareMetadata{
		JQExpression: ".id",
	}

	got, err := ApplyCompareRules(res, cmp)
	if err != nil {
		t.Fatalf("ApplyCompareRules: %v", err)
	}
	if got.V != "1" {
		t.Fatalf("expected jq to return id, got %#v", got.V)
	}
}
