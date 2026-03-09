package metadata

import (
	"context"
	"testing"
)

func TestResolveOperationSpecWithScopeResolvesPayloadTemplateHelpers(t *testing.T) {
	t.Parallel()

	spec, err := ResolveOperationSpecWithScope(
		context.Background(),
		ResourceMetadata{
			RemoteCollectionPath: "/api/{{payload_type .}}/customers",
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Path:        "/api/customers/{{.id}}",
					Accept:      "{{payload_media_type .}}",
					ContentType: "application/{{payload_type .}}",
					Query: map[string]string{
						"format":    "{{payload_type .}}",
						"extension": "{{payload_extension .}}",
					},
				},
			},
		},
		OperationGet,
		map[string]any{
			"id":               "acme",
			"payloadType":      "yaml",
			"payloadMediaType": "application/yaml",
			"payloadExtension": ".yaml",
		},
	)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
	}

	if spec.Path != "/api/customers/acme" {
		t.Fatalf("expected rendered path, got %q", spec.Path)
	}
	if spec.Accept != "application/yaml" {
		t.Fatalf("expected accept to resolve, got %q", spec.Accept)
	}
	if spec.ContentType != "application/yaml" {
		t.Fatalf("expected contentType to resolve, got %q", spec.ContentType)
	}
	if spec.Query["format"] != "yaml" {
		t.Fatalf("expected query.format to resolve, got %q", spec.Query["format"])
	}
	if spec.Query["extension"] != ".yaml" {
		t.Fatalf("expected query.extension to resolve, got %q", spec.Query["extension"])
	}
}
