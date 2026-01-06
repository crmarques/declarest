package openapi

import (
	"encoding/json"
	"testing"
)

func TestBuildResourceFromSpecUsesCollectionSchema(t *testing.T) {
	const specData = `
openapi: 3.0.0
paths:
  /items:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  default: default-name
                count:
                  type: number
                tags:
                  type: array
                  items:
                    type: string
`

	spec, err := ParseSpec([]byte(specData))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	res, err := BuildResourceFromSpec(spec, "/items/item-a")
	if err != nil {
		t.Fatalf("BuildResourceFromSpec: %v", err)
	}

	obj, ok := res.AsObject()
	if !ok {
		t.Fatal("expected object resource")
	}
	if name, ok := obj["name"].(string); !ok || name != "default-name" {
		t.Fatalf("expected name default, got %#v", obj["name"])
	}
	countValue, ok := obj["count"].(json.Number)
	if !ok || countValue.String() != "0" {
		t.Fatalf("expected count 0, got %#v", obj["count"])
	}
}

func TestBuildResourceFromSpecFallsBackToResourceSchema(t *testing.T) {
	const specData = `
openapi: 3.0.0
paths:
  /items/{id}:
    patch:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                status:
                  type: string
                  default: pending
`

	spec, err := ParseSpec([]byte(specData))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	res, err := BuildResourceFromSpec(spec, "/items/item-a")
	if err != nil {
		t.Fatalf("BuildResourceFromSpec: %v", err)
	}

	obj, ok := res.AsObject()
	if !ok {
		t.Fatal("expected object resource")
	}
	if status, ok := obj["status"].(string); !ok || status != "pending" {
		t.Fatalf("expected status default, got %#v", obj["status"])
	}
}

func TestBuildResourceFromSpecResolvesRefs(t *testing.T) {
	const specData = `
openapi: 3.0.0
components:
  schemas:
    itemPayload:
      type: object
      properties:
        title:
          type: string
          default: referenced
paths:
  /items:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/itemPayload"
`

	spec, err := ParseSpec([]byte(specData))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	res, err := BuildResourceFromSpec(spec, "/items/item-a")
	if err != nil {
		t.Fatalf("BuildResourceFromSpec: %v", err)
	}

	obj, ok := res.AsObject()
	if !ok {
		t.Fatal("expected object resource")
	}
	if title, ok := obj["title"].(string); !ok || title != "referenced" {
		t.Fatalf("expected title default from ref, got %#v", obj["title"])
	}
}
