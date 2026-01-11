package cmd

import (
	"reflect"
	"testing"
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
