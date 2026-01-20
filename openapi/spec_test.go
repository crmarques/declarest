package openapi

import "testing"

const sampleSpec = `{
  "openapi": "3.0.0",
  "paths": {
    "/items": {
      "get": {
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {}
            }
          }
        }
      },
      "post": {
        "requestBody": {
          "content": {
            "application/json": {}
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
    },
    "/items/{id}": {
      "patch": {
        "requestBody": {
          "content": {
            "application/merge-patch+json": {}
          }
        },
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    }
  }
}`

func TestParseSpecMatchPath(t *testing.T) {
	spec, err := ParseSpec([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	item := spec.MatchPath("/items/123")
	if item == nil {
		t.Fatalf("expected match for /items/123")
	}
	if item.Template != "/items/{id}" {
		t.Fatalf("expected template /items/{id}, got %q", item.Template)
	}

	op := item.Operation("patch")
	if op == nil {
		t.Fatalf("expected patch operation")
	}
	if len(op.RequestContentTypes) == 0 || op.RequestContentTypes[0] != "application/merge-patch+json" {
		t.Fatalf("unexpected request content types: %#v", op.RequestContentTypes)
	}
	if len(op.ResponseContentTypes) == 0 || op.ResponseContentTypes[0] != "application/json" {
		t.Fatalf("unexpected response content types: %#v", op.ResponseContentTypes)
	}
}
