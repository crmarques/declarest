package metadata_test

import (
	"strings"
	"testing"

	"declarest/internal/metadata"
	"declarest/internal/openapi"
	"declarest/internal/resource"
)

const inferenceSpecJSON = `
{
  "openapi": "3.0.0",
  "paths": {
    "/fruits": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "id": { "type": "string" },
                  "name": { "type": "string" }
                },
                "required": ["id", "name"]
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
    },
    "/fruits/{id}": {
      "get": {
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    },
    "/widgets": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "value": { "type": "string" }
                }
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
    },
    "/widgets/{widgetId}": {
      "get": {
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    },
    "/things": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "collectionId": { "type": "string" },
                  "displayName": { "type": "string" }
                },
                "required": ["collectionId"]
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
    },
    "/things/{thingId}": {
      "get": {
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
}
`

const keycloakSpecJSON = `
{
  "openapi": "3.0.0",
  "paths": {
    "/admin/realms/{realm}/clients": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "id": { "type": "string" },
                  "clientId": { "type": "string" }
                },
                "required": ["id", "clientId"]
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
    },
    "/admin/realms/{realm}": {
      "get": {
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {}
            }
          }
        }
      }
    },
    "/admin/realms/{realm}/clients/{id}": {
      "put": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "clientId": { "type": "string" },
                  "name": { "type": "string" }
                }
              }
            }
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
    },
    "/admin/realms/{realm}/user-registry/{storage}/mappers": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "id": { "type": "string" },
                  "name": { "type": "string" }
                },
                "required": ["id"]
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
    },
    "/admin/realms/{realm}/user-registry/{storage}/mappers/{id}": {
      "put": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" }
                }
              }
            }
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
}
`

const headerSpecJSON = `
{
  "openapi": "3.0.0",
  "paths": {
    "/widgets": {
      "get": {
        "parameters": [
          {
            "name": "X-Client",
            "in": "header",
            "schema": {
              "type": "string",
              "default": "clientA"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      },
      "post": {
        "parameters": [
          {
            "name": "X-Workspace",
            "in": "header",
            "example": "workspace"
          }
        ],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "created"
          }
        }
      }
    },
    "/widgets/{id}": {
      "get": {
        "parameters": [
          {
            "name": "X-Token",
            "in": "header",
            "schema": {
              "type": "string",
              "default": "token"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      },
      "put": {
        "parameters": [
          {
            "name": "X-Client",
            "in": "header",
            "schema": {
              "type": "string",
              "default": "clientA"
            }
          }
        ],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      },
      "delete": {
        "parameters": [
          {
            "name": "X-Delete",
            "in": "header",
            "schema": {
              "type": "string",
              "default": "delete"
            }
          }
        ],
        "responses": {
          "204": {
            "description": "deleted"
          }
        }
      }
    }
  }
}
`

func mustParseSpec(t *testing.T, data string) *openapi.Spec {
	t.Helper()
	spec, err := openapi.ParseSpec([]byte(data))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return spec
}

func reasonContains(reasons []string, substr string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, substr) {
			return true
		}
	}
	return false
}

func TestInferResourceMetadataChoosesIdAndAlias(t *testing.T) {
	spec := mustParseSpec(t, inferenceSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/fruits/apple", false, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "id" {
		t.Fatalf("expected idFromAttribute id, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "name" {
		t.Fatalf("expected aliasFromAttribute name, got %q", got)
	}
	if !reasonContains(result.Reasons, "schema property") {
		t.Fatalf("expected reasoning about schema properties, got %v", result.Reasons)
	}
}

func TestInferResourceMetadataOverrides(t *testing.T) {
	spec := mustParseSpec(t, inferenceSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/fruits/apple", false, metadata.InferenceOverrides{
		IDAttribute:    "uuid",
		AliasAttribute: "slug",
	})

	if got := result.ResourceInfo.IDFromAttribute; got != "uuid" {
		t.Fatalf("expected override idFromAttribute, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "slug" {
		t.Fatalf("expected override aliasFromAttribute, got %q", got)
	}
	if !reasonContains(result.Reasons, `forced to "uuid" via --id-from`) {
		t.Fatalf("expected id override reason, got %v", result.Reasons)
	}
	if !reasonContains(result.Reasons, `forced to "slug" via --alias-from`) {
		t.Fatalf("expected alias override reason, got %v", result.Reasons)
	}
}

func TestInferResourceMetadataFallsBackToPathParameter(t *testing.T) {
	spec := mustParseSpec(t, inferenceSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/widgets/widget-alpha", false, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "widgetId" {
		t.Fatalf("expected idFromAttribute widgetId, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "widgetId" {
		t.Fatalf("expected aliasFromAttribute widgetId, got %q", got)
	}
	if !reasonContains(result.Reasons, `path parameter "widgetId"`) {
		t.Fatalf("expected reason mentioning widgetId, got %v", result.Reasons)
	}
}

func TestInferCollectionMetadataUsesCollectionSchema(t *testing.T) {
	spec := mustParseSpec(t, inferenceSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/things", true, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "collectionId" {
		t.Fatalf("expected collection id attribute, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "displayName" {
		t.Fatalf("expected aliasFromAttribute displayName, got %q", got)
	}
}

func TestInferKeycloakClientMetadata(t *testing.T) {
	spec := mustParseSpec(t, keycloakSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/admin/realms/publico/clients/clientA", false, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "id" {
		t.Fatalf("expected idFromAttribute id, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "clientId" {
		t.Fatalf("expected aliasFromAttribute clientId, got %q", got)
	}
	if !reasonContains(result.Reasons, "clientId") {
		t.Fatalf("expected reason to mention clientId, got %v", result.Reasons)
	}
}

func TestInferKeycloakMapperCollectionMetadata(t *testing.T) {
	spec := mustParseSpec(t, keycloakSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/admin/realms/publico/user-registry/ldap-test/mappers/", true, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "id" {
		t.Fatalf("expected idFromAttribute id, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "name" {
		t.Fatalf("expected aliasFromAttribute name, got %q", got)
	}
	if !reasonContains(result.Reasons, "schema property \"name\"") {
		t.Fatalf("expected reason to mention schema property name, got %v", result.Reasons)
	}
}

func TestInferKeycloakRealmCollectionMetadata(t *testing.T) {
	spec := mustParseSpec(t, keycloakSpecJSON)
	result := metadata.InferResourceMetadata(spec, "/admin/realms", true, metadata.InferenceOverrides{})

	if got := result.ResourceInfo.IDFromAttribute; got != "realm" {
		t.Fatalf("expected idFromAttribute realm, got %q", got)
	}
	if got := result.ResourceInfo.AliasFromAttribute; got != "realm" {
		t.Fatalf("expected aliasFromAttribute realm, got %q", got)
	}
	if !reasonContains(result.Reasons, `path parameter "realm"`) {
		t.Fatalf("expected reason to mention path parameter realm, got %v", result.Reasons)
	}
}

func TestInferOperationHeaders(t *testing.T) {
	spec := mustParseSpec(t, headerSpecJSON)
	collectionResult := metadata.InferResourceMetadata(spec, "/widgets/", true, metadata.InferenceOverrides{})

	if collectionResult.OperationInfo == nil {
		t.Fatalf("expected operation info to be inferred for collection")
	}

	expectHeader := func(op *resource.OperationMetadata, expected string, label string) {
		if op == nil {
			t.Fatalf("expected %s metadata to exist", label)
		}
		if !headerListContains(op.HTTPHeaders, expected) {
			t.Fatalf("expected %s to include %q, got %#v", label, expected, op.HTTPHeaders)
		}
	}

	expectHeader(collectionResult.OperationInfo.ListCollection, "X-Client: clientA", "listCollection")
	expectHeader(collectionResult.OperationInfo.CreateResource, "X-Workspace: workspace", "createResource")

	resourceResult := metadata.InferResourceMetadata(spec, "/widgets/widgetA", false, metadata.InferenceOverrides{})
	if resourceResult.OperationInfo == nil {
		t.Fatalf("expected operation info to be inferred for resource")
	}

	expectHeader(resourceResult.OperationInfo.GetResource, "X-Token: token", "getResource")
	expectHeader(resourceResult.OperationInfo.UpdateResource, "X-Client: clientA", "updateResource")
	expectHeader(resourceResult.OperationInfo.DeleteResource, "X-Delete: delete", "deleteResource")

	if !reasonContains(collectionResult.Reasons, "X-Client") && !reasonContains(resourceResult.Reasons, "X-Client") {
		t.Fatalf("expected reason to mention inferred headers, got %v / %v", collectionResult.Reasons, resourceResult.Reasons)
	}
}

func headerListContains(headers resource.HeaderList, target string) bool {
	for _, entry := range headers {
		if entry == target {
			return true
		}
	}
	return false
}
