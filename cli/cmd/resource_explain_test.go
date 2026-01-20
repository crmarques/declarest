package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"
)

func TestPrintExplainIncludesOpenAPIDetails(t *testing.T) {
	const data = `{
		"openapi": "3.0.0",
		"paths": {
			"/items": {
				"get": {
					"summary": "List items",
					"responses": {
						"200": {
							"description": "OK",
							"content": {
								"application/json": {
									"schema": {
										"$ref": "#/components/schemas/ItemList"
									}
								}
							}
						}
					}
				},
				"post": {
					"summary": "Create an item",
					"description": "Adds a new item to the collection.",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"$ref": "#/components/schemas/Item"
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "Created"
						}
					}
				}
			}
		},
		"components": {
			"schemas": {
				"ItemList": {
					"type": "array",
					"items": {
						"$ref": "#/components/schemas/Item"
					}
				},
				"Item": {
					"type": "object",
					"properties": {
						"id": {
							"type": "string",
							"description": "Unique identifier"
						},
						"name": {
							"type": "string",
							"description": "Display name"
						}
					},
					"required": ["id", "name"]
				}
			}
		}
	}`

	spec, err := openapi.ParseSpec([]byte(data))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	record := resource.ResourceRecord{
		Path: "/items",
		Meta: resource.ResourceMetadata{
			ResourceInfo: &resource.ResourceInfoMetadata{
				CollectionPath: "/items",
			},
		},
	}

	var buf bytes.Buffer
	if err := printExplain(&buf, "/items", record, spec); err != nil {
		t.Fatalf("printExplain: %v", err)
	}
	output := buf.String()

	checkSubstr := func(substr string) {
		t.Helper()
		if !strings.Contains(output, substr) {
			t.Fatalf("output missing %q: %s", substr, output)
		}
	}

	checkSubstr("Template: /items")
	checkSubstr("GET:")
	checkSubstr("POST:")
	checkSubstr("requests: application/json")
	checkSubstr("response schema: ItemList")
	checkSubstr("request schema: Item")
	checkSubstr("Schema:")
	checkSubstr("ItemList:")
	checkSubstr("type=array")
	checkSubstr("properties:")
	checkSubstr("id:")
}
