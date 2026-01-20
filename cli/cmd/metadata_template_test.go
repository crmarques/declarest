package cmd

import (
	"encoding/json"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestMetadataEditTemplateShowsListCollectionFilterOptions(t *testing.T) {
	payload, err := formatMetadataOutput(resource.ResourceMetadata{})
	if err != nil {
		t.Fatalf("format metadata output: %v", err)
	}

	templateData, err := marshalMetadataEditPayloadWithComments(payload)
	if err != nil {
		t.Fatalf("marshal metadata payload: %v", err)
	}

	clean := stripMetadataComments(templateData)
	var meta map[string]any
	if err := json.Unmarshal(clean, &meta); err != nil {
		t.Fatalf("unmarshal template payload: %v", err)
	}

	opInfo, ok := meta["operationInfo"].(map[string]any)
	if !ok {
		t.Fatalf("operationInfo block missing from template")
	}
	list, ok := opInfo["listCollection"].(map[string]any)
	if !ok {
		t.Fatalf("listCollection block missing from template")
	}

	if _, ok := list["jqFilter"]; !ok {
		t.Fatalf("jqFilter missing from listCollection template")
	}

	payloadMap, ok := list["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload block missing from listCollection template")
	}
	if _, ok := payloadMap["filterAttributes"]; !ok {
		t.Fatalf("filterAttributes missing from payload template")
	}
	if _, ok := payloadMap["suppressAttributes"]; !ok {
		t.Fatalf("suppressAttributes missing from payload template")
	}
	if _, ok := payloadMap["jqExpression"]; !ok {
		t.Fatalf("jqExpression missing from payload template")
	}
}

func TestStripDefaultMetadataRemovesListCollectionFilterDefaults(t *testing.T) {
	defaults := map[string]any{}
	applyListCollectionTemplateDefaults(defaults)

	current := map[string]any{}
	applyListCollectionTemplateDefaults(current)

	stripped := stripDefaultMetadata(current, defaults)
	if len(stripped) != 0 {
		t.Fatalf("expected defaults to be stripped, got %#v", stripped)
	}
}
