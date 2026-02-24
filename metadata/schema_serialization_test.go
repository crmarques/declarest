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
			string(OperationCreate): {
				Path:        "/api/customers",
				Accept:      "application/json",
				ContentType: "application/json",
			},
			string(OperationGet): {
				Path: "/api/customers/{{.id}}",
			},
			string(OperationList): {
				Path: "/api/customers",
			},
			string(OperationCompare): {
				Suppress: []string{"/updatedAt"},
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
	createResource, ok := operationInfo["createResource"].(map[string]any)
	if !ok {
		t.Fatalf("expected createResource operation entry, got %#v", operationInfo["createResource"])
	}
	if _, hasAccept := createResource["accept"]; hasAccept {
		t.Fatalf("expected createResource.accept to be omitted, got %#v", createResource["accept"])
	}
	if _, hasContentType := createResource["contentType"]; hasContentType {
		t.Fatalf("expected createResource.contentType to be omitted, got %#v", createResource["contentType"])
	}
	httpHeaders, ok := createResource["httpHeaders"].([]any)
	if !ok {
		t.Fatalf("expected createResource.httpHeaders array, got %#v", createResource["httpHeaders"])
	}
	assertHeader := func(name string, value string) {
		t.Helper()
		for _, item := range httpHeaders {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if entry["name"] == name && entry["value"] == value {
				return
			}
		}
		t.Fatalf("expected createResource.httpHeaders to contain %q=%q, got %#v", name, value, httpHeaders)
	}
	assertHeader("Accept", "application/json")
	assertHeader("Content-Type", "application/json")
	compareResource, ok := operationInfo["compareResources"].(map[string]any)
	if !ok {
		t.Fatalf("expected compareResources operation entry, got %#v", operationInfo["compareResources"])
	}
	if _, hasIgnoreAttributes := compareResource["ignoreAttributes"]; hasIgnoreAttributes {
		t.Fatalf("expected compareResources.ignoreAttributes to be omitted, got %#v", compareResource)
	}
	compareSuppress, ok := compareResource["suppressAttributes"].([]any)
	if !ok || len(compareSuppress) != 1 || compareSuppress[0] != "/updatedAt" {
		t.Fatalf("expected compareResources.suppressAttributes [/updatedAt], got %#v", compareResource["suppressAttributes"])
	}

	defaults, ok := operationInfo["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("expected operationInfo.defaults object, got %#v", operationInfo["defaults"])
	}
	payload, ok := defaults["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected operationInfo.defaults.payload object, got %#v", defaults["payload"])
	}
	filter, ok := payload["filterAttributes"].([]any)
	if !ok || len(filter) != 0 {
		t.Fatalf("expected explicit empty defaults.payload.filterAttributes array, got %#v", payload["filterAttributes"])
	}
	suppress, ok := payload["suppressAttributes"].([]any)
	if !ok || len(suppress) != 1 || suppress[0] != "/updatedAt" {
		t.Fatalf("expected defaults.payload.suppressAttributes [/updatedAt], got %#v", payload["suppressAttributes"])
	}
	if jq, ok := payload["jqExpression"].(string); !ok || jq != "" {
		t.Fatalf("expected explicit empty defaults.payload.jqExpression, got %#v", payload["jqExpression"])
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
		      "payload": {
		        "filterAttributes": [],
		        "suppressAttributes": ["/updatedAt"],
		        "jqExpression": ""
		      }
		    },
		    "createResource": {
		      "httpMethod": "POST",
		      "path": "/admin/realms"
		    },
		    "getResource": {
		      "httpMethod": "GET",
		      "path": "/admin/realms/{{.realm}}",
		      "httpHeaders": [
		        {"name": "X-Tenant", "value": "platform"}
		      ],
		      "payload": {
		        "filterAttributes": []
		      }
		    },
		    "compareResources": {
		      "ignoreAttributes": ["/updatedAt"]
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
		if decoded.Operations[string(OperationGet)].Method != "GET" {
			t.Fatalf("unexpected get method: %#v", decoded.Operations[string(OperationGet)])
		}
		if !reflect.DeepEqual(decoded.Operations[string(OperationGet)].Headers, map[string]string{"X-Tenant": "platform"}) {
			t.Fatalf("unexpected get headers: %#v", decoded.Operations[string(OperationGet)].Headers)
		}
		if decoded.Operations[string(OperationCreate)].Method != "POST" {
			t.Fatalf("unexpected create method: %#v", decoded.Operations[string(OperationCreate)])
		}
		if decoded.Filter == nil || len(decoded.Filter) != 0 {
			t.Fatalf("expected explicit empty filter, got %#v", decoded.Filter)
		}
		if !reflect.DeepEqual(decoded.Suppress, []string{"/updatedAt"}) {
			t.Fatalf("unexpected suppress: %#v", decoded.Suppress)
		}
		if !reflect.DeepEqual(decoded.Operations[string(OperationCompare)].Suppress, []string{"/updatedAt"}) {
			t.Fatalf("unexpected compare suppress from ignoreAttributes alias: %#v", decoded.Operations[string(OperationCompare)].Suppress)
		}
	})
}

func TestResourceMetadataUnmarshalJSONSupportsOperationURLPathSyntax(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "resourceInfo": {
	    "collectionPath": "/admin/realms/{{.realm}}/components"
	  },
	  "operationInfo": {
	    "getResource": {
	      "url": {
	        "path": "./{{.id}}"
	      }
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if decoded.CollectionPath != "/admin/realms/{{.realm}}/components" {
		t.Fatalf("unexpected collectionPath: %q", decoded.CollectionPath)
	}
	if decoded.Operations[string(OperationGet)].Path != "./{{.id}}" {
		t.Fatalf("unexpected get operation path: %#v", decoded.Operations[string(OperationGet)])
	}
}

func TestResourceMetadataUnmarshalJSONSupportsScalarPayloadTransformAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operationInfo": {
	    "defaults": {
	      "payload": {
	        "filterAttributes": "name"
	      }
	    },
	    "createResource": {
	      "payload": {
	        "suppressAttributes": "secret"
	      }
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if !reflect.DeepEqual(decoded.Filter, []string{"name"}) {
		t.Fatalf("expected scalar defaults payload filter to decode as single-item list, got %#v", decoded.Filter)
	}
	createSpec := decoded.Operations[string(OperationCreate)]
	if !reflect.DeepEqual(createSpec.Suppress, []string{"secret"}) {
		t.Fatalf("expected scalar create payload suppress to decode as single-item list, got %#v", createSpec.Suppress)
	}
}

func TestResourceMetadataUnmarshalJSONPromotesMediaHeadersFromHTTPHeaders(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operationInfo": {
	    "createResource": {
	      "httpMethod": "POST",
	      "path": "/api/customers",
	      "httpHeaders": [
	        {"name": "Accept", "value": "application/yaml"},
	        {"name": "Content-Type", "value": "application/yaml"},
	        {"name": "X-Tenant", "value": "platform"}
	      ]
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	createSpec := decoded.Operations[string(OperationCreate)]
	if createSpec.Accept != "application/yaml" {
		t.Fatalf("expected create accept promoted from httpHeaders, got %q", createSpec.Accept)
	}
	if createSpec.ContentType != "application/yaml" {
		t.Fatalf("expected create contentType promoted from httpHeaders, got %q", createSpec.ContentType)
	}
	if !reflect.DeepEqual(createSpec.Headers, map[string]string{"X-Tenant": "platform"}) {
		t.Fatalf("expected non-media headers to remain in Headers, got %#v", createSpec.Headers)
	}
}
