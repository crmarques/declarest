package resource

import (
	"encoding/json"
	"testing"
)

func TestResourceYAMLRoundTripPreservesNumbers(t *testing.T) {
	res, err := NewResource(map[string]any{
		"count": 42,
		"ratio": 3.14,
		"name":  "example",
	})
	if err != nil {
		t.Fatalf("NewResource: %v", err)
	}

	payload, err := res.MarshalYAMLBytes()
	if err != nil {
		t.Fatalf("MarshalYAMLBytes: %v", err)
	}

	parsed, err := NewResourceFromYAML(payload)
	if err != nil {
		t.Fatalf("NewResourceFromYAML: %v", err)
	}

	obj, ok := parsed.AsObject()
	if !ok {
		t.Fatalf("expected object, got %#v", parsed.V)
	}
	if _, ok := obj["count"].(json.Number); !ok {
		t.Fatalf("expected count to be json.Number, got %T", obj["count"])
	}
	if _, ok := obj["ratio"].(json.Number); !ok {
		t.Fatalf("expected ratio to be json.Number, got %T", obj["ratio"])
	}
	if obj["name"] != "example" {
		t.Fatalf("expected name to be example, got %#v", obj["name"])
	}
}
