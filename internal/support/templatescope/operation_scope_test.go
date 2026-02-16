package templatescope

import "testing"

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
