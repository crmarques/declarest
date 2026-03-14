package metadata

import (
	"testing"
)

func TestDescribeResourceBasicMetadata(t *testing.T) {
	t.Parallel()

	md := ResourceMetadata{
		ID:                   "{{/id}}",
		Alias:                "{{/clientId}}",
		Format:               "json",
		RemoteCollectionPath: "/admin/realms/{{/realm}}/clients",
		Operations: map[string]OperationSpec{
			"get":    {Method: "GET", Path: "/admin/realms/{{/realm}}/clients/{{/clientId}}"},
			"create": {Method: "POST", Path: "/admin/realms/{{/realm}}/clients"},
			"update": {Method: "PUT", Path: "/admin/realms/{{/realm}}/clients/{{/clientId}}"},
			"delete": {Method: "DELETE", Path: "/admin/realms/{{/realm}}/clients/{{/clientId}}"},
			"list":   {Method: "GET", Path: "/admin/realms/{{/realm}}/clients"},
		},
	}

	desc := DescribeResource("/realms/master/clients/", md, nil)

	if desc.Path != "/realms/master/clients/" {
		t.Fatalf("expected path /realms/master/clients/, got %q", desc.Path)
	}
	if desc.Identity == nil {
		t.Fatal("expected identity to be present")
	}
	if desc.Identity.ID != "{{/id}}" {
		t.Fatalf("expected identity id, got %q", desc.Identity.ID)
	}
	if desc.Format != "json" {
		t.Fatalf("expected format json, got %q", desc.Format)
	}
	if len(desc.Operations) != 5 {
		t.Fatalf("expected 5 operations, got %d", len(desc.Operations))
	}
}

func TestDescribeResourceWithOpenAPISchema(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients": map[string]any{
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"required": []any{
										"clientId",
										"protocol",
									},
									"properties": map[string]any{
										"clientId": map[string]any{
											"type":        "string",
											"description": "Client identifier",
										},
										"name": map[string]any{
											"type": "string",
										},
										"enabled": map[string]any{
											"type":    "boolean",
											"default": true,
										},
										"protocol": map[string]any{
											"type": "string",
											"enum": []any{"openid-connect", "saml"},
										},
										"redirectUris": map[string]any{
											"type": "array",
											"items": map[string]any{
												"type": "string",
											},
										},
										"attributes": map[string]any{
											"type": "object",
											"additionalProperties": map[string]any{
												"type": "string",
											},
										},
									},
								},
							},
						},
					},
				},
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "array",
										"items": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"id": map[string]any{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			"create": {Method: "POST", Path: "/admin/realms/{{/realm}}/clients"},
			"list":   {Method: "GET", Path: "/admin/realms/{{/realm}}/clients"},
		},
	}

	desc := DescribeResource("/realms/master/clients/", md, openAPISpec)

	if len(desc.Schemas) == 0 {
		t.Fatal("expected at least one schema")
	}

	schema := desc.Schemas[0]
	if schema.Operation != "create" {
		t.Fatalf("expected create operation schema, got %q", schema.Operation)
	}
	if schema.Method != "POST" {
		t.Fatalf("expected POST method, got %q", schema.Method)
	}
	if schema.Type != "object" {
		t.Fatalf("expected object type, got %q", schema.Type)
	}
	if len(schema.Properties) != 6 {
		t.Fatalf("expected 6 properties, got %d", len(schema.Properties))
	}

	// Verify required fields
	propMap := make(map[string]SchemaNode)
	for _, p := range schema.Properties {
		propMap[p.Name] = p
	}

	if !propMap["clientId"].Required {
		t.Fatal("expected clientId to be required")
	}
	if !propMap["protocol"].Required {
		t.Fatal("expected protocol to be required")
	}
	if propMap["name"].Required {
		t.Fatal("expected name to not be required")
	}

	// Verify types
	if propMap["clientId"].Type != "string" {
		t.Fatalf("expected clientId type string, got %q", propMap["clientId"].Type)
	}
	if propMap["enabled"].Type != "boolean" {
		t.Fatalf("expected enabled type boolean, got %q", propMap["enabled"].Type)
	}
	if propMap["redirectUris"].Type != "string[]" {
		t.Fatalf("expected redirectUris type string[], got %q", propMap["redirectUris"].Type)
	}
	if propMap["attributes"].Type != "map[string]string" {
		t.Fatalf("expected attributes type map[string]string, got %q", propMap["attributes"].Type)
	}

	// Verify enum
	if len(propMap["protocol"].Enum) != 2 {
		t.Fatalf("expected 2 enum values for protocol, got %d", len(propMap["protocol"].Enum))
	}

	// Verify default
	if propMap["enabled"].Default != true {
		t.Fatalf("expected enabled default true, got %v", propMap["enabled"].Default)
	}

	// Verify description
	if propMap["clientId"].Description != "Client identifier" {
		t.Fatalf("expected clientId description, got %q", propMap["clientId"].Description)
	}
}

func TestDescribeResourceWithSchemaRef(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"components": map[string]any{
			"schemas": map[string]any{
				"Job": map[string]any{
					"type":     "object",
					"required": []any{"name"},
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
						"schedule": map[string]any{
							"$ref": "#/components/schemas/Schedule",
						},
					},
				},
				"Schedule": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"cron": map[string]any{
							"type":    "string",
							"pattern": "^[0-9*/ ]+$",
						},
						"enabled": map[string]any{
							"type": "boolean",
						},
					},
				},
			},
		},
		"paths": map[string]any{
			"/api/jobs": map[string]any{
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/Job",
								},
							},
						},
					},
				},
			},
		},
	}

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			"create": {Method: "POST", Path: "/api/jobs"},
		},
	}

	desc := DescribeResource("/jobs/", md, openAPISpec)

	if len(desc.Schemas) == 0 {
		t.Fatal("expected schema from $ref resolution")
	}

	schema := desc.Schemas[0]
	if len(schema.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(schema.Properties))
	}

	propMap := make(map[string]SchemaNode)
	for _, p := range schema.Properties {
		propMap[p.Name] = p
	}

	if !propMap["name"].Required {
		t.Fatal("expected name to be required")
	}

	// Verify nested object from $ref
	schedule := propMap["schedule"]
	if schedule.Type != "object" {
		t.Fatalf("expected schedule type object, got %q", schedule.Type)
	}
	if len(schedule.Properties) != 2 {
		t.Fatalf("expected 2 schedule properties, got %d", len(schedule.Properties))
	}
}

func TestDescribeResourceWithAllOf(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"components": map[string]any{
			"schemas": map[string]any{
				"Base": map[string]any{
					"type":     "object",
					"required": []any{"id"},
					"properties": map[string]any{
						"id": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
		"paths": map[string]any{
			"/api/items": map[string]any{
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"allOf": []any{
										map[string]any{
											"$ref": "#/components/schemas/Base",
										},
										map[string]any{
											"type":     "object",
											"required": []any{"name"},
											"properties": map[string]any{
												"name": map[string]any{
													"type": "string",
												},
												"value": map[string]any{
													"type": "integer",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			"create": {Method: "POST", Path: "/api/items"},
		},
	}

	desc := DescribeResource("/items/", md, openAPISpec)

	if len(desc.Schemas) == 0 {
		t.Fatal("expected schema from allOf merge")
	}

	schema := desc.Schemas[0]
	if len(schema.Properties) != 3 {
		t.Fatalf("expected 3 properties from allOf merge, got %d", len(schema.Properties))
	}

	propMap := make(map[string]SchemaNode)
	for _, p := range schema.Properties {
		propMap[p.Name] = p
	}

	if !propMap["id"].Required {
		t.Fatal("expected id from Base to be required")
	}
	if !propMap["name"].Required {
		t.Fatal("expected name to be required")
	}
	if propMap["value"].Required {
		t.Fatal("expected value to not be required")
	}
}

func TestDescribeResourceFallsBackToResponseSchema(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/api/items/{id}": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"id": map[string]any{
												"type": "string",
											},
											"name": map[string]any{
												"type": "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			"get": {Method: "GET", Path: "/api/items/{{/id}}"},
		},
	}

	desc := DescribeResource("/items/my-item", md, openAPISpec)

	if len(desc.Schemas) == 0 {
		t.Fatal("expected fallback response schema")
	}

	schema := desc.Schemas[0]
	if schema.Source != "response" {
		t.Fatalf("expected response source, got %q", schema.Source)
	}
	if len(schema.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(schema.Properties))
	}
}

func TestDescribeResourceNoOpenAPI(t *testing.T) {
	t.Parallel()

	md := ResourceMetadata{
		ID:     "{{/id}}",
		Format: "json",
		Operations: map[string]OperationSpec{
			"get": {Method: "GET", Path: "/api/items/{{/id}}"},
		},
	}

	desc := DescribeResource("/items/my-item", md, nil)

	if desc.Identity == nil {
		t.Fatal("expected identity even without OpenAPI")
	}
	if len(desc.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(desc.Operations))
	}
	if len(desc.Schemas) != 0 {
		t.Fatalf("expected no schemas without OpenAPI, got %d", len(desc.Schemas))
	}
}

func TestFindPathItemForMetadataOperation(t *testing.T) {
	t.Parallel()

	pathItems := map[string]map[string]any{
		"/admin/realms/{realm}/clients":               {"get": map[string]any{}, "post": map[string]any{}},
		"/admin/realms/{realm}/clients/{client-uuid}": {"get": map[string]any{}, "put": map[string]any{}},
	}

	item, found := findPathItemForMetadataOperation("/admin/realms/{{/realm}}/clients", pathItems)
	if !found {
		t.Fatal("expected to find path item for collection path")
	}
	if _, hasGet := item["get"]; !hasGet {
		t.Fatal("expected path item to have GET method")
	}

	item, found = findPathItemForMetadataOperation("/admin/realms/{{/realm}}/clients/{{/clientId}}", pathItems)
	if !found {
		t.Fatal("expected to find path item for resource path")
	}
	if _, hasPut := item["put"]; !hasPut {
		t.Fatal("expected path item to have PUT method")
	}

	_, found = findPathItemForMetadataOperation("/nonexistent/path", pathItems)
	if found {
		t.Fatal("expected not to find nonexistent path")
	}
}
