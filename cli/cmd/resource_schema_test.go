package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"
)

func TestSchemaLinesFormatsProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"required": []any{
			"id",
			"profile",
		},
		"properties": map[string]any{
			"id": map[string]any{
				"type": "string",
			},
			"profile": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"email": map[string]any{
						"type":   "string",
						"format": "email",
					},
					"age": map[string]any{
						"type": "integer",
					},
				},
			},
		},
	}

	got := schemaLines(schema)
	want := []string{
		"type=object",
		"required: id, profile",
		"properties:",
		"  id:",
		"    type=string",
		"  profile:",
		"    type=object",
		"    properties:",
		"      age:",
		"        type=integer",
		"      email:",
		"        type=string format=email",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schemaLines returned\n%v\nwant\n%v", got, want)
	}
}

func TestSchemaLinesIncludeDescription(t *testing.T) {
	schema := map[string]any{
		"type":        "object",
		"description": "Represents an item",
	}

	got := schemaLines(schema)
	want := []string{
		"type=object",
		"description: Represents an item",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schemaLines returned\n%v\nwant\n%v", got, want)
	}
}

func TestDescribeOpenAPIShowsSummaryAndDescription(t *testing.T) {
	const specJSON = `{
  "openapi": "3.0.0",
  "paths": {
    "/items": {
      "get": {
        "summary": "List items",
        "description": "Retrieve all items",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "properties": {
                      "id": {
                        "type": "string"
                      }
                    }
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "summary": "Create item",
        "description": "Create a single item",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "description": "Item payload",
                "type": "object"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "created",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    }
  }
}`

	spec, err := openapi.ParseSpec([]byte(specJSON))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	var buf bytes.Buffer
	describeOpenAPI(&buf, spec, resource.ResourceRecord{}, "/items", false)
	output := buf.String()

	for _, substring := range []string{
		"Template: /items",
		"summary: List items",
		"description: Retrieve all items",
		"summary: Create item",
		"description: Create a single item",
		"requests:",
		"responses: application/json",
		"response schema: type=array",
		"request schema: type=object",
		"Schema is not defined for this path.",
	} {
		if !strings.Contains(output, substring) {
			t.Fatalf("expected %q in output:\n%s", substring, output)
		}
	}
}
