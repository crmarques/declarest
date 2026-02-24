package metadata

import "testing"

func TestResolveResourceFormatTemplatesInMetadata(t *testing.T) {
	t.Parallel()

	input := ResourceMetadata{
		CollectionPath: "/api/{{resource_format .}}/customers",
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:        "/api/customers/{{.id}}",
				Accept:      "application/{{resource_format .}}",
				ContentType: "application/{{resource_format .}}",
				Query: map[string]string{
					"format": "{{resource_format .}}",
				},
			},
		},
	}

	resolved, err := ResolveResourceFormatTemplatesInMetadata(input, "yaml")
	if err != nil {
		t.Fatalf("ResolveResourceFormatTemplatesInMetadata returned error: %v", err)
	}

	if resolved.CollectionPath != "/api/yaml/customers" {
		t.Fatalf("expected collectionPath to resolve resource_format token, got %q", resolved.CollectionPath)
	}
	getSpec := resolved.Operations[string(OperationGet)]
	if getSpec.Path != "/api/customers/{{.id}}" {
		t.Fatalf("expected non-resource_format template to be preserved, got %q", getSpec.Path)
	}
	if getSpec.Accept != "application/yaml" {
		t.Fatalf("expected accept to resolve, got %q", getSpec.Accept)
	}
	if getSpec.ContentType != "application/yaml" {
		t.Fatalf("expected contentType to resolve, got %q", getSpec.ContentType)
	}
	if getSpec.Query["format"] != "yaml" {
		t.Fatalf("expected query.format to resolve, got %q", getSpec.Query["format"])
	}
}
