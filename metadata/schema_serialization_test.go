package metadata

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestResourceMetadataMarshalJSONUsesNestedSchema(t *testing.T) {
	t.Parallel()

	value := ResourceMetadata{
		IDFromAttribute:       "id",
		AliasFromAttribute:    "name",
		CollectionPath:        "/api/customers",
		SecretsFromAttributes: []string{"credentials.password"},
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path: "/api/customers/{{.id}}",
			},
			string(OperationList): {
				Path: "/api/customers",
			},
		},
		Filter:   []string{},
		Suppress: []string{"/updatedAt"},
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded payload returned error: %v", err)
	}

	if _, found := decoded["idFromAttribute"]; found {
		t.Fatalf("expected nested metadata schema without flat idFromAttribute, got %#v", decoded)
	}

	resourceInfo, ok := decoded["resourceInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected resourceInfo object, got %#v", decoded["resourceInfo"])
	}
	if resourceInfo["idFromAttribute"] != "id" {
		t.Fatalf("expected resourceInfo.idFromAttribute=id, got %#v", resourceInfo["idFromAttribute"])
	}
	if resourceInfo["aliasFromAttribute"] != "name" {
		t.Fatalf("expected resourceInfo.aliasFromAttribute=name, got %#v", resourceInfo["aliasFromAttribute"])
	}
	if resourceInfo["collectionPath"] != "/api/customers" {
		t.Fatalf("expected resourceInfo.collectionPath=/api/customers, got %#v", resourceInfo["collectionPath"])
	}
	secretAttributes, ok := resourceInfo["secretInAttributes"].([]any)
	if !ok || len(secretAttributes) != 1 || secretAttributes[0] != "credentials.password" {
		t.Fatalf("expected resourceInfo.secretInAttributes, got %#v", resourceInfo["secretInAttributes"])
	}

	operationInfo, ok := decoded["operationInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected operationInfo object, got %#v", decoded["operationInfo"])
	}
	if _, hasLegacyGet := operationInfo["get"]; hasLegacyGet {
		t.Fatalf("expected canonical operationInfo keys, got %#v", operationInfo)
	}
	if _, hasGetResource := operationInfo["getResource"]; !hasGetResource {
		t.Fatalf("expected getResource operation entry, got %#v", operationInfo)
	}
	if _, hasListCollection := operationInfo["listCollection"]; !hasListCollection {
		t.Fatalf("expected listCollection operation entry, got %#v", operationInfo)
	}

	defaults, ok := operationInfo["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("expected operationInfo.defaults object, got %#v", operationInfo["defaults"])
	}
	filter, ok := defaults["filter"].([]any)
	if !ok || len(filter) != 0 {
		t.Fatalf("expected explicit empty defaults.filter array, got %#v", defaults["filter"])
	}
	suppress, ok := defaults["suppress"].([]any)
	if !ok || len(suppress) != 1 || suppress[0] != "/updatedAt" {
		t.Fatalf("expected defaults.suppress [/updatedAt], got %#v", defaults["suppress"])
	}
}

func TestResourceMetadataUnmarshalJSONSupportsLegacyAndNestedSchemas(t *testing.T) {
	t.Parallel()

	t.Run("legacy_flat_schema", func(t *testing.T) {
		t.Parallel()

		payload := []byte(`{
		  "idFromAttribute": "id",
		  "aliasFromAttribute": "name",
		  "secretsFromAttributes": ["password"],
		  "operations": {
		    "get": {"path": "/api/customers/{{.id}}"},
		    "list": {"path": "/api/customers"}
		  },
		  "filter": ["/items"],
		  "suppress": ["/updatedAt"],
		  "jq": "."
		}`)

		var decoded ResourceMetadata
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("unmarshal returned error: %v", err)
		}

		if decoded.IDFromAttribute != "id" || decoded.AliasFromAttribute != "name" {
			t.Fatalf("unexpected identity fields: %+v", decoded)
		}
		if !reflect.DeepEqual(decoded.SecretsFromAttributes, []string{"password"}) {
			t.Fatalf("unexpected secretsFromAttributes: %#v", decoded.SecretsFromAttributes)
		}
		if decoded.Operations[string(OperationGet)].Path != "/api/customers/{{.id}}" {
			t.Fatalf("unexpected get operation: %#v", decoded.Operations[string(OperationGet)])
		}
		if decoded.Operations[string(OperationList)].Path != "/api/customers" {
			t.Fatalf("unexpected list operation: %#v", decoded.Operations[string(OperationList)])
		}
		if !reflect.DeepEqual(decoded.Filter, []string{"/items"}) {
			t.Fatalf("unexpected filter: %#v", decoded.Filter)
		}
		if !reflect.DeepEqual(decoded.Suppress, []string{"/updatedAt"}) {
			t.Fatalf("unexpected suppress: %#v", decoded.Suppress)
		}
		if decoded.JQ != "." {
			t.Fatalf("unexpected jq value: %q", decoded.JQ)
		}
	})

	t.Run("nested_schema", func(t *testing.T) {
		t.Parallel()

		payload := []byte(`{
		  "resourceInfo": {
		    "idFromAttribute": "realm",
		    "aliasFromAttribute": "realm",
		    "collectionPath": "/admin/realms",
		    "secretInAttributes": []
		  },
		  "operationInfo": {
		    "defaults": {
		      "filter": [],
		      "suppress": ["/updatedAt"]
		    },
		    "createResource": {
		      "path": "/admin/realms"
		    },
		    "getResource": {
		      "path": "/admin/realms/{{.realm}}"
		    }
		  }
		}`)

		var decoded ResourceMetadata
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("unmarshal returned error: %v", err)
		}

		if decoded.IDFromAttribute != "realm" || decoded.AliasFromAttribute != "realm" {
			t.Fatalf("unexpected identity fields: %+v", decoded)
		}
		if decoded.CollectionPath != "/admin/realms" {
			t.Fatalf("unexpected collectionPath: %q", decoded.CollectionPath)
		}
		if decoded.SecretsFromAttributes == nil || len(decoded.SecretsFromAttributes) != 0 {
			t.Fatalf("expected explicit empty secret attributes, got %#v", decoded.SecretsFromAttributes)
		}
		if decoded.Operations[string(OperationCreate)].Path != "/admin/realms" {
			t.Fatalf("unexpected create operation: %#v", decoded.Operations[string(OperationCreate)])
		}
		if decoded.Operations[string(OperationGet)].Path != "/admin/realms/{{.realm}}" {
			t.Fatalf("unexpected get operation: %#v", decoded.Operations[string(OperationGet)])
		}
		if decoded.Filter == nil || len(decoded.Filter) != 0 {
			t.Fatalf("expected explicit empty filter, got %#v", decoded.Filter)
		}
		if !reflect.DeepEqual(decoded.Suppress, []string{"/updatedAt"}) {
			t.Fatalf("unexpected suppress: %#v", decoded.Suppress)
		}
	})
}
