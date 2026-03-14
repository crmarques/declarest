package metadata

import (
	"encoding/json"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestResourceMetadataMarshalJSONUsesNestedSchema(t *testing.T) {
	t.Parallel()

	value := ResourceMetadata{
		Selector:             &SelectorSpec{Descendants: boolPointer(true)},
		ID:                   "{{/id}}",
		Alias:                "{{/name}}",
		RequiredAttributes:   []string{"/name", "/realm"},
		RemoteCollectionPath: "/api/customers",
		Secret:               boolPointer(true),
		SecretAttributes:     []string{"/credentials/password"},
		Operations: map[string]OperationSpec{
			string(OperationCreate): {
				Path:        "/api/customers",
				Accept:      "application/json",
				ContentType: "application/json",
				Validate: &OperationValidationSpec{
					RequiredAttributes: []string{},
					Assertions: []ValidationAssertion{
						{
							Message: "name must be present",
							JQ:      `has("name")`,
						},
					},
					SchemaRef: "openapi:request-body",
				},
			},
			string(OperationGet): {
				Path: "/api/customers/{{/id}}",
			},
			string(OperationList): {
				Path: "/api/customers",
			},
			string(OperationCompare): {
				Transforms: []TransformStep{
					{ExcludeAttributes: []string{"/updatedAt"}},
				},
			},
		},
		Transforms: []TransformStep{
			{SelectAttributes: []string{}},
			{ExcludeAttributes: []string{"/updatedAt"}},
			{JQExpression: ""},
		},
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded payload returned error: %v", err)
	}

	if _, found := decoded["idAttribute"]; found {
		t.Fatalf("expected nested metadata schema without flat idAttribute, got %#v", decoded)
	}
	selector, ok := decoded["selector"].(map[string]any)
	if !ok {
		t.Fatalf("expected selector object, got %#v", decoded["selector"])
	}
	if selector["descendants"] != true {
		t.Fatalf("expected selector.descendants=true, got %#v", selector["descendants"])
	}

	resource, ok := decoded["resource"].(map[string]any)
	if !ok {
		t.Fatalf("expected resource object, got %#v", decoded["resource"])
	}
	if resource["id"] != "{{/id}}" {
		t.Fatalf("expected resource.id={{/id}}, got %#v", resource["id"])
	}
	if resource["alias"] != "{{/name}}" {
		t.Fatalf("expected resource.alias={{/name}}, got %#v", resource["alias"])
	}
	requiredAttributes, ok := resource["requiredAttributes"].([]any)
	if !ok || len(requiredAttributes) != 2 || requiredAttributes[0] != "/name" || requiredAttributes[1] != "/realm" {
		t.Fatalf("expected resource.requiredAttributes, got %#v", resource["requiredAttributes"])
	}
	if resource["remoteCollectionPath"] != "/api/customers" {
		t.Fatalf("expected resource.remoteCollectionPath=/api/customers, got %#v", resource["remoteCollectionPath"])
	}
	if _, found := resource["collectionPath"]; found {
		t.Fatalf("expected legacy resource.collectionPath to be omitted, got %#v", resource["collectionPath"])
	}
	if resource["secret"] != true {
		t.Fatalf("expected resource.secret=true, got %#v", resource["secret"])
	}
	secretAttributes, ok := resource["secretAttributes"].([]any)
	if !ok || len(secretAttributes) != 1 || secretAttributes[0] != "/credentials/password" {
		t.Fatalf("expected resource.secretAttributes, got %#v", resource["secretAttributes"])
	}

	operations, ok := decoded["operations"].(map[string]any)
	if !ok {
		t.Fatalf("expected operations object, got %#v", decoded["operations"])
	}
	if _, hasLegacyGet := operations["getResource"]; hasLegacyGet {
		t.Fatalf("expected canonical operations keys, got %#v", operations)
	}
	if _, hasGet := operations["get"]; !hasGet {
		t.Fatalf("expected get operation entry, got %#v", operations)
	}
	if _, hasList := operations["list"]; !hasList {
		t.Fatalf("expected list operation entry, got %#v", operations)
	}
	create, ok := operations["create"].(map[string]any)
	if !ok {
		t.Fatalf("expected create operation entry, got %#v", operations["create"])
	}
	if _, hasAccept := create["accept"]; hasAccept {
		t.Fatalf("expected create.accept to be omitted, got %#v", create["accept"])
	}
	if _, hasContentType := create["contentType"]; hasContentType {
		t.Fatalf("expected create.contentType to be omitted, got %#v", create["contentType"])
	}
	headers, ok := create["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected create.headers object, got %#v", create["headers"])
	}
	if headers["Accept"] != "application/json" {
		t.Fatalf("expected create.headers.Accept application/json, got %#v", headers["Accept"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("expected create.headers.Content-Type application/json, got %#v", headers["Content-Type"])
	}
	validateValue, ok := create["validate"].(map[string]any)
	if !ok {
		t.Fatalf("expected create.validate object, got %#v", create["validate"])
	}
	createRequiredAttributes, ok := validateValue["requiredAttributes"].([]any)
	if !ok || len(createRequiredAttributes) != 0 {
		t.Fatalf("expected explicit empty validate.requiredAttributes array, got %#v", validateValue["requiredAttributes"])
	}
	assertions, ok := validateValue["assertions"].([]any)
	if !ok || len(assertions) != 1 {
		t.Fatalf("expected one validate assertion, got %#v", validateValue["assertions"])
	}
	assertion, ok := assertions[0].(map[string]any)
	if !ok {
		t.Fatalf("expected assertion object, got %#v", assertions[0])
	}
	if assertion["message"] != "name must be present" || assertion["jq"] != `has("name")` {
		t.Fatalf("unexpected assertion payload %#v", assertion)
	}
	if validateValue["schemaRef"] != "openapi:request-body" {
		t.Fatalf("expected validate.schemaRef openapi:request-body, got %#v", validateValue["schemaRef"])
	}
	compareResource, ok := operations["compare"].(map[string]any)
	if !ok {
		t.Fatalf("expected compare operation entry, got %#v", operations["compare"])
	}
	if _, hasIgnoreAttributes := compareResource["ignoreAttributes"]; hasIgnoreAttributes {
		t.Fatalf("expected compare.ignoreAttributes to be omitted, got %#v", compareResource)
	}
	compareMutation, ok := compareResource["transforms"].([]any)
	if !ok || len(compareMutation) != 1 {
		t.Fatalf("expected compare.transforms, got %#v", compareResource["transforms"])
	}
	compareStep, ok := compareMutation[0].(map[string]any)
	if !ok {
		t.Fatalf("expected compare transforms object, got %#v", compareMutation[0])
	}
	compareSuppress, ok := compareStep["excludeAttributes"].([]any)
	if !ok || len(compareSuppress) != 1 || compareSuppress[0] != "/updatedAt" {
		t.Fatalf("expected compare transforms excludeAttributes [/updatedAt], got %#v", compareStep["excludeAttributes"])
	}

	defaults, ok := operations["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("expected operations.defaults object, got %#v", operations["defaults"])
	}
	transforms, ok := defaults["transforms"].([]any)
	if !ok || len(transforms) != 3 {
		t.Fatalf("expected operations.defaults.transforms array, got %#v", defaults["transforms"])
	}
	filterStep, ok := transforms[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first transforms step object, got %#v", transforms[0])
	}
	filter, ok := filterStep["selectAttributes"].([]any)
	if !ok || len(filter) != 0 {
		t.Fatalf("expected explicit empty defaults transforms selectAttributes array, got %#v", filterStep["selectAttributes"])
	}
	suppressStep, ok := transforms[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second transforms step object, got %#v", transforms[1])
	}
	suppress, ok := suppressStep["excludeAttributes"].([]any)
	if !ok || len(suppress) != 1 || suppress[0] != "/updatedAt" {
		t.Fatalf("expected defaults transforms excludeAttributes [/updatedAt], got %#v", suppressStep["excludeAttributes"])
	}
	jqStep, ok := transforms[2].(map[string]any)
	if !ok {
		t.Fatalf("expected third transforms step object, got %#v", transforms[2])
	}
	if jq, ok := jqStep["jqExpression"].(string); !ok || jq != "" {
		t.Fatalf("expected explicit empty defaults transforms jqExpression, got %#v", jqStep["jqExpression"])
	}
}

func TestResourceMetadataMarshalJSONPreservesPointerShorthand(t *testing.T) {
	t.Parallel()

	value := ResourceMetadata{
		ID:    "/id",
		Alias: "/name",
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded payload returned error: %v", err)
	}

	resource, ok := decoded["resource"].(map[string]any)
	if !ok {
		t.Fatalf("expected resource object, got %#v", decoded["resource"])
	}
	if resource["id"] != "/id" {
		t.Fatalf("expected resource.id=/id, got %#v", resource["id"])
	}
	if resource["alias"] != "/name" {
		t.Fatalf("expected resource.alias=/name, got %#v", resource["alias"])
	}
}

func TestResourceMetadataWholeResourceSecretRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(ResourceMetadata{
		Secret: boolPointer(true),
	})
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	var decoded ResourceMetadata
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if !decoded.IsWholeResourceSecret() {
		t.Fatalf("expected whole-resource secret round-trip, got %#v", decoded)
	}
}

func TestResourceMetadataSelectorRoundTrip(t *testing.T) {
	t.Parallel()

	value := ResourceMetadata{
		Selector: &SelectorSpec{Descendants: boolPointer(true)},
	}

	jsonEncoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal returned error: %v", err)
	}

	var jsonDecoded ResourceMetadata
	if err := json.Unmarshal(jsonEncoded, &jsonDecoded); err != nil {
		t.Fatalf("json unmarshal returned error: %v", err)
	}
	if !jsonDecoded.Selector.AllowsDescendants() {
		t.Fatalf("expected json selector round-trip, got %#v", jsonDecoded.Selector)
	}

	yamlEncoded, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("yaml marshal returned error: %v", err)
	}

	var yamlDecoded ResourceMetadata
	if err := yaml.Unmarshal(yamlEncoded, &yamlDecoded); err != nil {
		t.Fatalf("yaml unmarshal returned error: %v", err)
	}
	if !yamlDecoded.Selector.AllowsDescendants() {
		t.Fatalf("expected yaml selector round-trip, got %#v", yamlDecoded.Selector)
	}
}

func TestResourceMetadataUnmarshalJSONRejectsLegacySchemaAndSupportsNestedSchema(t *testing.T) {
	t.Parallel()

	t.Run("rejects_legacy_flat_schema", func(t *testing.T) {
		t.Parallel()

		payload := []byte(`{
		  "idAttribute": "/id",
		  "aliasAttribute": "/name",
		  "secretsFromAttributes": ["password"],
		  "operations": {
		    "get": {"path": "/api/customers/{{/id}}"},
		    "list": {"path": "/api/customers"}
		  },
		  "filter": ["/items"],
		  "suppress": ["/updatedAt"],
		  "jq": "."
		}`)

		var decoded ResourceMetadata
		if err := json.Unmarshal(payload, &decoded); err == nil {
			t.Fatal("expected legacy flat schema to be rejected")
		}
	})

	t.Run("nested_schema", func(t *testing.T) {
		t.Parallel()

		payload := []byte(`{
		  "resource": {
		    "id": "{{/realm}}",
		    "alias": "{{/realm}}",
		    "requiredAttributes": ["/realm", "/displayName"],
		    "remoteCollectionPath": "/admin/realms",
		    "secretAttributes": []
		  },
		  "operations": {
		    "defaults": {
		      "transforms": [
		        {"selectAttributes": []},
		        {"excludeAttributes": ["/updatedAt"]},
		        {"jqExpression": ""}
		      ]
		    },
		    "create": {
		      "method": "POST",
		      "path": "/admin/realms",
		      "validate": {
		        "requiredAttributes": ["/realm"],
		        "schemaRef": "openapi:request-body"
		      }
		    },
		    "get": {
		      "method": "GET",
		      "path": "/admin/realms/{{/realm}}",
		      "headers": [
		        {"X-Tenant": "platform"}
		      ],
		      "validate": {
		        "assertions": [
		          {
		            "message": "realm must be a non-empty string",
		            "jq": "has(\"realm\") and (.realm | type==\"string\") and (.realm | length > 0)"
		          }
		        ]
		      },
		      "transforms": [
		        {"selectAttributes": []}
		      ]
		    },
		    "compare": {
		      "transforms": [
		        {"excludeAttributes": ["/updatedAt"]}
		      ]
		    }
		  }
		}`)

		var decoded ResourceMetadata
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("unmarshal returned error: %v", err)
		}

		if decoded.ID != "{{/realm}}" || decoded.Alias != "{{/realm}}" {
			t.Fatalf("unexpected identity fields: %+v", decoded)
		}
		if !reflect.DeepEqual(decoded.RequiredAttributes, []string{"/realm", "/displayName"}) {
			t.Fatalf("unexpected resource.requiredAttributes: %#v", decoded.RequiredAttributes)
		}
		if decoded.RemoteCollectionPath != "/admin/realms" {
			t.Fatalf("unexpected remoteCollectionPath: %q", decoded.RemoteCollectionPath)
		}
		if decoded.SecretAttributes == nil || len(decoded.SecretAttributes) != 0 {
			t.Fatalf("expected explicit empty secret attributes, got %#v", decoded.SecretAttributes)
		}
		if decoded.Operations[string(OperationCreate)].Path != "/admin/realms" {
			t.Fatalf("unexpected create operation: %#v", decoded.Operations[string(OperationCreate)])
		}
		if decoded.Operations[string(OperationGet)].Path != "/admin/realms/{{/realm}}" {
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
		createValidate := decoded.Operations[string(OperationCreate)].Validate
		if createValidate == nil {
			t.Fatal("expected create validate block to be decoded")
		}
		if !reflect.DeepEqual(createValidate.RequiredAttributes, []string{"/realm"}) {
			t.Fatalf("unexpected create validate.requiredAttributes: %#v", createValidate.RequiredAttributes)
		}
		if createValidate.SchemaRef != "openapi:request-body" {
			t.Fatalf("unexpected create validate.schemaRef: %q", createValidate.SchemaRef)
		}
		getValidate := decoded.Operations[string(OperationGet)].Validate
		if getValidate == nil || len(getValidate.Assertions) != 1 {
			t.Fatalf("expected get validate assertions to be decoded, got %#v", getValidate)
		}
		if getValidate.Assertions[0].Message != "realm must be a non-empty string" {
			t.Fatalf("unexpected get validate assertion message: %#v", getValidate.Assertions[0].Message)
		}
		if getValidate.Assertions[0].JQ == "" {
			t.Fatal("expected get validate assertion jq to be populated")
		}
		if decoded.Transforms == nil || len(decoded.Transforms) != 3 {
			t.Fatalf("expected explicit default transforms pipeline, got %#v", decoded.Transforms)
		}
		if !reflect.DeepEqual(decoded.Transforms[1].ExcludeAttributes, []string{"/updatedAt"}) {
			t.Fatalf("unexpected default transforms suppress step: %#v", decoded.Transforms)
		}
		if !reflect.DeepEqual(
			decoded.Operations[string(OperationCompare)].Transforms,
			[]TransformStep{{ExcludeAttributes: []string{"/updatedAt"}}},
		) {
			t.Fatalf("unexpected compare transforms: %#v", decoded.Operations[string(OperationCompare)].Transforms)
		}
	})
}

func TestResourceMetadataUnmarshalJSONRejectsLegacyCollectionPathField(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "resource": {
	    "collectionPath": "/admin/realms/{{/realm}}/components"
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err == nil {
		t.Fatal("expected resource.collectionPath to be rejected")
	}
}

func TestResourceMetadataUnmarshalYAMLRejectsLegacyCollectionPathField(t *testing.T) {
	t.Parallel()

	payload := []byte(`
resource:
  collectionPath: /admin/realms/{{/realm}}/components
`)

	var decoded ResourceMetadata
	if err := yaml.Unmarshal(payload, &decoded); err == nil {
		t.Fatal("expected resource.collectionPath to be rejected")
	}
}

func TestResourceMetadataMarshalJSONIncludesExternalizedAttributes(t *testing.T) {
	t.Parallel()

	value := ResourceMetadata{
		ExternalizedAttributes: []ExternalizedAttribute{
			{
				Path:           "/script",
				File:           "script.sh",
				Template:       "{{include %s}}",
				Mode:           "text",
				SaveBehavior:   "externalize",
				RenderBehavior: "include",
				Enabled:        boolPointer(true),
			},
		},
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded payload returned error: %v", err)
	}

	resource, ok := decoded["resource"].(map[string]any)
	if !ok {
		t.Fatalf("expected resource object, got %#v", decoded["resource"])
	}

	items, ok := resource["externalizedAttributes"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one externalizedAttributes entry, got %#v", resource["externalizedAttributes"])
	}
	entry, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected externalizedAttributes entry object, got %#v", items[0])
	}
	pathValue, ok := entry["path"].(string)
	if !ok || pathValue != "/script" {
		t.Fatalf("expected path /script, got %#v", entry["path"])
	}
	if entry["file"] != "script.sh" {
		t.Fatalf("expected file script.sh, got %#v", entry["file"])
	}
	if entry["enabled"] != true {
		t.Fatalf("expected enabled=true, got %#v", entry["enabled"])
	}
}

func TestResourceMetadataUnmarshalJSONSupportsExternalizedAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "resource": {
	    "externalizedAttributes": [
	      {
	        "path": "/spec/template/script",
	        "file": "script.sh",
	        "enabled": false
	      }
	    ]
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if len(decoded.ExternalizedAttributes) != 1 {
		t.Fatalf("expected one externalized attribute, got %#v", decoded.ExternalizedAttributes)
	}
	entry := decoded.ExternalizedAttributes[0]
	if entry.Path != "/spec/template/script" {
		t.Fatalf("unexpected path %#v", entry.Path)
	}
	if entry.File != "script.sh" {
		t.Fatalf("unexpected file %q", entry.File)
	}
	if entry.Enabled == nil || *entry.Enabled {
		t.Fatalf("expected enabled=false, got %#v", entry.Enabled)
	}
}

func TestResourceMetadataUnmarshalJSONRejectsOperationURLPathSyntax(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "resource": {
	    "remoteCollectionPath": "/admin/realms/{{/realm}}/components"
	  },
	  "operations": {
	    "get": {
	      "url": {
	        "path": "./{{/id}}"
	      }
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err == nil {
		t.Fatal("expected operations.get.url.path to be rejected")
	}
}

func TestResourceMetadataUnmarshalJSONRejectsLegacyCompareTransformAlias(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operations": {
	    "compare": {
	      "ignoreAttributes": ["/updatedAt"]
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err == nil {
		t.Fatal("expected compare.ignoreAttributes to be rejected")
	}
}

func TestResourceMetadataUnmarshalJSONSupportsScalarPayloadTransformAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operations": {
	    "defaults": {
	      "transforms": [
	        {"selectAttributes": "/name"}
	      ]
	    },
	    "create": {
	      "transforms": [
	        {"excludeAttributes": "/secret"}
	      ]
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if !reflect.DeepEqual(
		decoded.Transforms,
		[]TransformStep{{SelectAttributes: []string{"/name"}}},
	) {
		t.Fatalf("expected scalar defaults transforms selectAttributes to decode as single-item list, got %#v", decoded.Transforms)
	}
	createSpec := decoded.Operations[string(OperationCreate)]
	if !reflect.DeepEqual(
		createSpec.Transforms,
		[]TransformStep{{ExcludeAttributes: []string{"/secret"}}},
	) {
		t.Fatalf("expected scalar create transforms excludeAttributes to decode as single-item list, got %#v", createSpec.Transforms)
	}
}

func TestResourceMetadataUnmarshalJSONSupportsScalarValidateRequiredAttributes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operations": {
	    "create": {
	      "validate": {
	        "requiredAttributes": "/realm"
	      }
	    }
	  }
	}`)

	var decoded ResourceMetadata
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	createValidate := decoded.Operations[string(OperationCreate)].Validate
	if createValidate == nil {
		t.Fatal("expected create validate block to be decoded")
	}
	if !reflect.DeepEqual(createValidate.RequiredAttributes, []string{"/realm"}) {
		t.Fatalf("expected scalar requiredAttributes to decode as single-item list, got %#v", createValidate.RequiredAttributes)
	}
}

func TestResourceMetadataUnmarshalJSONPromotesMediaHeadersFromHTTPHeaders(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
	  "operations": {
	    "create": {
	      "method": "POST",
	      "path": "/api/customers",
	      "headers": [
	        {"Accept": "application/yaml"},
	        {"Content-Type": "application/yaml"},
	        {"X-Tenant": "platform"}
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
		t.Fatalf("expected create accept promoted from headers, got %q", createSpec.Accept)
	}
	if createSpec.ContentType != "application/yaml" {
		t.Fatalf("expected create contentType promoted from headers, got %q", createSpec.ContentType)
	}
	if !reflect.DeepEqual(createSpec.Headers, map[string]string{"X-Tenant": "platform"}) {
		t.Fatalf("expected non-media headers to remain in Headers, got %#v", createSpec.Headers)
	}
}

func TestResourceMetadataFormatJSONRoundtrip(t *testing.T) {
	t.Parallel()

	original := ResourceMetadata{
		ID:     "{{/id}}",
		Format: "yaml",
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	var decoded ResourceMetadata
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if decoded.Format != "yaml" {
		t.Fatalf("expected format=yaml, got %q", decoded.Format)
	}
	if decoded.ID != "{{/id}}" {
		t.Fatalf("expected id={{/id}}, got %q", decoded.ID)
	}
}

func TestResourceMetadataFormatOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	original := ResourceMetadata{
		ID: "{{/id}}",
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	raw := map[string]any{}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	resourceObj, ok := raw["resource"].(map[string]any)
	if !ok {
		t.Fatalf("expected resource object, got %#v", raw["resource"])
	}
	if _, found := resourceObj["format"]; found {
		t.Fatalf("expected format to be omitted when empty, got %#v", resourceObj)
	}
}

func TestResourceMetadataLegacyFormatFieldsAreRejected(t *testing.T) {
	t.Parallel()

	for _, payload := range []string{
		`{"resource":{"preferredFormat":"yaml"}}`,
		`{"resource":{"payloadType":"yaml"}}`,
		`{"resource":{"defaultFormat":"yaml"}}`,
	} {
		if _, err := DecodeResourceMetadataJSON([]byte(payload)); err == nil {
			t.Fatalf("expected legacy field payload to be rejected: %s", payload)
		}
	}
}
