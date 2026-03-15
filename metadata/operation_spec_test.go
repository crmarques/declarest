// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func TestResolveOperationSpecMergesAndRenders(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveOperationSpec(context.Background(), ResourceMetadata{
		Transforms: []TransformStep{
			{SelectAttributes: []string{"/root"}},
			{ExcludeAttributes: []string{"/internal"}},
		},
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:    "/api/customers/{{/id}}",
				Headers: map[string]string{"X-Tenant": "{{/tenant}}"},
				Query:   map[string]string{"expand": "{{/expand}}"},
			},
		},
	}, OperationGet, map[string]any{
		"id":     "acme",
		"tenant": "north",
		"expand": "true",
	})
	if err != nil {
		t.Fatalf("ResolveOperationSpec returned error: %v", err)
	}

	if resolved.Path != "/api/customers/acme" {
		t.Fatalf("expected rendered path, got %q", resolved.Path)
	}
	if resolved.Headers["X-Tenant"] != "north" {
		t.Fatalf("expected rendered header, got %+v", resolved.Headers)
	}
	if resolved.Query["expand"] != "true" {
		t.Fatalf("expected rendered query, got %+v", resolved.Query)
	}
	if len(resolved.Transforms) != 2 {
		t.Fatalf("expected inherited transforms pipeline, got %+v", resolved.Transforms)
	}
	if TransformStepType(resolved.Transforms[0]) != "selectAttributes" ||
		len(resolved.Transforms[0].SelectAttributes) != 1 ||
		resolved.Transforms[0].SelectAttributes[0] != "/root" {
		t.Fatalf("expected first transforms step to select /root, got %+v", resolved.Transforms[0])
	}
	if TransformStepType(resolved.Transforms[1]) != "excludeAttributes" ||
		len(resolved.Transforms[1].ExcludeAttributes) != 1 ||
		resolved.Transforms[1].ExcludeAttributes[0] != "/internal" {
		t.Fatalf("expected second transforms step to suppress /internal, got %+v", resolved.Transforms[1])
	}
}

func TestResolveOperationSpecSupportsPointerAndBareKeyTemplates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "json_pointer", path: "/api/customers/{{/id}}"},
		{name: "single_level_shorthand", path: "/api/customers/{{id}}"},
		{name: "legacy_dot_notation", path: "/api/customers/{{.id}}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := ResolveOperationSpec(context.Background(), ResourceMetadata{
				Operations: map[string]OperationSpec{
					string(OperationGet): {
						Path: test.path,
					},
				},
			}, OperationGet, map[string]any{"id": "acme"})
			if err != nil {
				t.Fatalf("ResolveOperationSpec returned error: %v", err)
			}
			if resolved.Path != "/api/customers/acme" {
				t.Fatalf("expected rendered path, got %q", resolved.Path)
			}
		})
	}
}

func TestResolveOperationSpecValidation(t *testing.T) {
	t.Parallel()

	_, err := ResolveOperationSpec(context.Background(), ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {},
		},
	}, OperationGet, map[string]any{"id": "acme"})
	assertValidationError(t, err)
}

func TestResolveOperationSpecWithScopeSupportsRemoteCollectionPathIndirection(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveOperationSpecWithScope(
		context.Background(),
		ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Path: "./{{/id}}",
				},
			},
		},
		OperationGet,
		map[string]any{
			"realm":                 "platform",
			"id":                    "123456",
			"logicalCollectionPath": "/admin/realms/platform/user-registry",
			"remoteCollectionPath":  "/admin/realms/platform/user-registry",
		},
	)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
	}

	if resolved.Path != "/admin/realms/platform/components/123456" {
		t.Fatalf("unexpected resolved path %q", resolved.Path)
	}
}

func TestResolveOperationSpecWithScopeDefaultsOperationPathTemplates(t *testing.T) {
	t.Parallel()

	metadata := ResourceMetadata{
		RemoteCollectionPath: "/admin/realms/{{/realm}}/components",
	}
	scope := map[string]any{
		"realm": "platform",
		"id":    "abc",
	}

	createSpec, err := ResolveOperationSpecWithScope(context.Background(), metadata, OperationCreate, scope)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope(create) returned error: %v", err)
	}
	if createSpec.Path != "/admin/realms/platform/components" {
		t.Fatalf("expected create default to resolve collection path, got %q", createSpec.Path)
	}

	listSpec, err := ResolveOperationSpecWithScope(context.Background(), metadata, OperationList, scope)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope(list) returned error: %v", err)
	}
	if listSpec.Path != "/admin/realms/platform/components" {
		t.Fatalf("expected list default to resolve collection path, got %q", listSpec.Path)
	}

	getSpec, err := ResolveOperationSpecWithScope(context.Background(), metadata, OperationGet, scope)
	if err != nil {
		t.Fatalf("ResolveOperationSpecWithScope(get) returned error: %v", err)
	}
	if getSpec.Path != "/admin/realms/platform/components/abc" {
		t.Fatalf("expected get default to resolve collection item path, got %q", getSpec.Path)
	}
}

func TestResolveOperationSpecWithScopeSupportsPayloadTemplateFunc(t *testing.T) {
	t.Parallel()

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:        "/api/customers/{{/id}}",
				Accept:      "{{payload_media_type .}}",
				ContentType: "application/{{payload_type .}}",
			},
		},
	}

	t.Run("defaults_to_json", func(t *testing.T) {
		t.Parallel()

		spec, err := ResolveOperationSpecWithScope(context.Background(), md, OperationGet, map[string]any{
			"id": "acme",
		})
		if err != nil {
			t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
		}
		if spec.Accept != "" {
			t.Fatalf("expected empty accept without concrete payload context, got %q", spec.Accept)
		}
		if spec.ContentType != "application/" {
			t.Fatalf("expected contentType template to render without payload_type fallback, got %q", spec.ContentType)
		}
	})

	t.Run("uses_yaml_when_provided", func(t *testing.T) {
		t.Parallel()

		spec, err := ResolveOperationSpecWithScope(context.Background(), md, OperationGet, map[string]any{
			"id":          "acme",
			"payloadType": "yaml",
		})
		if err != nil {
			t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
		}
		if spec.Accept != "application/yaml" {
			t.Fatalf("expected accept application/yaml, got %q", spec.Accept)
		}
	})
}

func TestResolveOperationSpecWithScopeRejectsInvalidPayloadTemplateUsage(t *testing.T) {
	t.Parallel()

	_, err := ResolveOperationSpecWithScope(context.Background(), ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:   "/api/customers/{{/id}}",
				Accept: "application/{{payload_type \"yaml\"}}",
			},
		},
	}, OperationGet, map[string]any{
		"id":          "acme",
		"payloadType": "yaml",
	})
	assertValidationError(t, err)
}

func TestInferFromOpenAPIDefaults(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPI(context.Background(), "/customers/acme", InferenceRequest{})
	if err != nil {
		t.Fatalf("InferFromOpenAPI returned error: %v", err)
	}

	getOperation := inferred.Operations[string(OperationGet)]
	if getOperation.Method != "GET" || getOperation.Path != "/customers/acme" {
		t.Fatalf("unexpected inferred get operation: %+v", getOperation)
	}

	listOperation := inferred.Operations[string(OperationList)]
	if listOperation.Method != "GET" || listOperation.Path != "/customers" {
		t.Fatalf("unexpected inferred list operation: %+v", listOperation)
	}
}

func TestInferFromOpenAPISupportsIntermediarySelectors(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms/_/clients/",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/clients": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/clients/{clientId}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/clientId}}" {
		t.Fatalf("expected alias to be inferred as {{/clientId}}, got %q", inferred.Alias)
	}
	if len(inferred.SecretAttributes) != 1 || inferred.SecretAttributes[0] != "/secret" {
		t.Fatalf("expected inferred secret attribute [secret], got %#v", inferred.SecretAttributes)
	}

	listOperation := inferred.Operations[string(OperationList)]
	if listOperation.Path != "/admin/realms/{{/realm}}/clients" {
		t.Fatalf("unexpected inferred list operation path: %+v", listOperation)
	}

	getOperation := inferred.Operations[string(OperationGet)]
	if getOperation.Path != "/admin/realms/{{/realm}}/clients/{{/clientId}}" {
		t.Fatalf("unexpected inferred get operation path: %+v", getOperation)
	}
}

func TestInferFromOpenAPIPrefersSchemaIdentityAttributesOverMissingPathVariable(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms/_/organizations/",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get": map[string]any{
						"responses": map[string]any{
							"200": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"$ref": "#/components/schemas/OrganizationRepresentation",
										},
									},
								},
							},
						},
					},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
			"components": map[string]any{
				"schemas": map[string]any{
					"OrganizationRepresentation": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":    map[string]any{"type": "string"},
							"alias": map[string]any{"type": "string"},
							"name":  map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/alias}}" {
		t.Fatalf("expected alias to be inferred as {{/alias}}, got %q", inferred.Alias)
	}
}

func TestInferFromOpenAPIPrefersSchemaIdentityAttributesForNonTemplateSafePathVariable(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms/_/organizations/",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization-id}": map[string]any{
					"get": map[string]any{
						"responses": map[string]any{
							"200": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"$ref": "#/components/schemas/OrganizationRepresentation",
										},
									},
								},
							},
						},
					},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
			"components": map[string]any{
				"schemas": map[string]any{
					"OrganizationRepresentation": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":    map[string]any{"type": "string"},
							"alias": map[string]any{"type": "string"},
							"name":  map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/alias}}" {
		t.Fatalf("expected alias to be inferred as {{/alias}}, got %q", inferred.Alias)
	}
}

func TestInferFromOpenAPISupportsConcreteSegmentsAgainstTemplatedOpenAPIPaths(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms/publico-br/organizations/",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get": map[string]any{
						"responses": map[string]any{
							"200": map[string]any{
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"$ref": "#/components/schemas/OrganizationRepresentation",
										},
									},
								},
							},
						},
					},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
			"components": map[string]any{
				"schemas": map[string]any{
					"OrganizationRepresentation": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":    map[string]any{"type": "string"},
							"alias": map[string]any{"type": "string"},
							"name":  map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/alias}}" {
		t.Fatalf("expected alias to be inferred as {{/alias}}, got %q", inferred.Alias)
	}
}

func TestInferFromOpenAPIFallsBackToCollectionResponseSchemaForIdentity(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms/publico-br/organizations/",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms/{realm}/organizations": map[string]any{
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
													"id":    map[string]any{"type": "string"},
													"alias": map[string]any{"type": "string"},
													"name":  map[string]any{"type": "string"},
												},
											},
										},
									},
								},
							},
						},
					},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}/organizations/{organization}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/alias}}" {
		t.Fatalf("expected alias to be inferred as {{/alias}}, got %q", inferred.Alias)
	}
}

func TestInferFromOpenAPITreatsCollectionPathWithoutSelectorAsCollection(t *testing.T) {
	t.Parallel()

	inferred, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms",
		InferenceRequest{},
		map[string]any{
			"paths": map[string]any{
				"/admin/realms": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/admin/realms/{realm}": map[string]any{
					"get":    map[string]any{},
					"put":    map[string]any{},
					"delete": map[string]any{},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/realm}}" {
		t.Fatalf("expected id to be inferred as {{/realm}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/realm}}" {
		t.Fatalf("expected alias to be inferred as {{/realm}}, got %q", inferred.Alias)
	}

	listOperation := inferred.Operations[string(OperationList)]
	if listOperation.Path != "/admin/realms" {
		t.Fatalf("unexpected inferred list operation path: %+v", listOperation)
	}

	getOperation := inferred.Operations[string(OperationGet)]
	if getOperation.Path != "/admin/realms/{{/realm}}" {
		t.Fatalf("unexpected inferred get operation path: %+v", getOperation)
	}
}

func TestInferFromOpenAPISetsOperationValidationFromRequestBodySchema(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms": map[string]any{
				"get": map[string]any{},
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"required": []any{
										"realm",
										"displayName",
									},
									"properties": map[string]any{
										"realm":       map[string]any{"type": "string"},
										"displayName": map[string]any{"type": "string"},
									},
								},
							},
						},
					},
				},
			},
			"/admin/realms/{realm}": map[string]any{
				"get": map[string]any{},
				"put": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"allOf": []any{
										map[string]any{
											"type": "object",
											"required": []any{
												"enabled",
											},
										},
										map[string]any{
											"type": "object",
											"required": []any{
												"displayName",
											},
										},
									},
								},
							},
						},
					},
				},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/admin/realms", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	createValidation := inferred.Operations[string(OperationCreate)].Validate
	if createValidation == nil {
		t.Fatal("expected inferred create validate block")
	}
	if createValidation.SchemaRef != "openapi:request-body" {
		t.Fatalf("expected create validate.schemaRef, got %#v", createValidation.SchemaRef)
	}
	if len(createValidation.RequiredAttributes) != 2 ||
		createValidation.RequiredAttributes[0] != "/displayName" ||
		createValidation.RequiredAttributes[1] != "/realm" {
		t.Fatalf("unexpected create validate.requiredAttributes %#v", createValidation.RequiredAttributes)
	}

	updateValidation := inferred.Operations[string(OperationUpdate)].Validate
	if updateValidation == nil {
		t.Fatal("expected inferred update validate block")
	}
	if updateValidation.SchemaRef != "openapi:request-body" {
		t.Fatalf("expected update validate.schemaRef, got %#v", updateValidation.SchemaRef)
	}
	if len(updateValidation.RequiredAttributes) != 2 ||
		updateValidation.RequiredAttributes[0] != "/displayName" ||
		updateValidation.RequiredAttributes[1] != "/enabled" {
		t.Fatalf("unexpected update validate.requiredAttributes %#v", updateValidation.RequiredAttributes)
	}
}

func TestInferFromOpenAPIInfersOctetStreamFormat(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/files": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/octet-stream": map[string]any{
									"schema": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/octet-stream": map[string]any{
								"schema": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							},
						},
					},
				},
			},
			"/files/{id}": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/octet-stream": map[string]any{
									"schema": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"put": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/octet-stream": map[string]any{
								"schema": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							},
						},
					},
				},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/files", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.Format != resource.PayloadTypeOctetStream {
		t.Fatalf("expected octet-stream format, got %q", inferred.Format)
	}
	if inferred.Operations[string(OperationCreate)].Validate != nil {
		t.Fatalf("expected binary create validation to be omitted, got %#v", inferred.Operations[string(OperationCreate)].Validate)
	}
	if inferred.Operations[string(OperationUpdate)].Validate != nil {
		t.Fatalf("expected binary update validation to be omitted, got %#v", inferred.Operations[string(OperationUpdate)].Validate)
	}
}

func TestCompactInferredMetadataDefaultsOmitsFallbackOperations(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms": map[string]any{
				"get":  map[string]any{},
				"post": map[string]any{},
			},
			"/admin/realms/{realm}": map[string]any{
				"get":    map[string]any{},
				"put":    map[string]any{},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/admin/realms", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	compact, err := CompactInferredMetadataDefaults("/admin/realms", inferred, openAPISpec)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}

	if compact.ID != "{{/realm}}" {
		t.Fatalf("expected id to be preserved, got %q", compact.ID)
	}
	if compact.Alias != "{{/realm}}" {
		t.Fatalf("expected alias to be preserved, got %q", compact.Alias)
	}
	if len(compact.Operations) != 0 {
		t.Fatalf("expected fallback-equivalent operations to be omitted, got %#v", compact.Operations)
	}
}

func TestCompactInferredMetadataDefaultsOmitsOpenAPIDefaultOperationsWithNonTemplateSafeParams(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients": map[string]any{
				"get":  map[string]any{},
				"post": map[string]any{},
			},
			"/admin/realms/{realm}/clients/{client-uuid}": map[string]any{
				"get":    map[string]any{},
				"put":    map[string]any{},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/admin/realms/_/clients/", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	if inferred.ID != "{{/id}}" {
		t.Fatalf("expected id to be inferred as {{/id}}, got %q", inferred.ID)
	}
	if inferred.Alias != "{{/clientId}}" {
		t.Fatalf("expected alias to be inferred as {{/clientId}}, got %q", inferred.Alias)
	}
	if len(inferred.SecretAttributes) != 1 || inferred.SecretAttributes[0] != "/secret" {
		t.Fatalf("expected inferred secret attribute [secret], got %#v", inferred.SecretAttributes)
	}

	compact, err := CompactInferredMetadataDefaults("/admin/realms/_/clients/", inferred, openAPISpec)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}

	if len(compact.Operations) != 0 {
		t.Fatalf("expected openapi-default operations to be omitted, got %#v", compact.Operations)
	}
}

func TestCompactInferredMetadataDefaultsOmitsOpenAPIValidationDefaults(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms": map[string]any{
				"get": map[string]any{},
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"required": []any{
										"realm",
									},
								},
							},
						},
					},
				},
			},
			"/admin/realms/{realm}": map[string]any{
				"get": map[string]any{},
				"put": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"required": []any{
										"realm",
									},
								},
							},
						},
					},
				},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/admin/realms", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	compact, err := CompactInferredMetadataDefaults("/admin/realms", inferred, openAPISpec)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}
	if len(compact.Operations) != 0 {
		t.Fatalf("expected openapi validation defaults to be omitted, got %#v", compact.Operations)
	}
}

func TestCompactInferredMetadataDefaultsOmitsOpenAPIFormatDefaults(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/files": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/octet-stream": map[string]any{
									"schema": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/octet-stream": map[string]any{
								"schema": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							},
						},
					},
				},
			},
			"/files/{id}": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/octet-stream": map[string]any{
									"schema": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"put": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/octet-stream": map[string]any{
								"schema": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							},
						},
					},
				},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/files", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}

	compact, err := CompactInferredMetadataDefaults("/files", inferred, openAPISpec)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}

	if compact.Format != "" {
		t.Fatalf("expected inferred format default to be omitted, got %q", compact.Format)
	}
}

func TestInferFromOpenAPISpecUsesAnyFormatForMixedPayloadTypes(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/files": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "array",
										"items": map[string]any{
											"type": "object",
										},
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
			"/files/{id}": map[string]any{
				"get": map[string]any{
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/xml": map[string]any{
									"schema": map[string]any{
										"type": "string",
									},
								},
							},
						},
					},
				},
				"put": map[string]any{
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/xml": map[string]any{
								"schema": map[string]any{
									"type": "string",
								},
							},
						},
					},
				},
				"delete": map[string]any{},
			},
		},
	}

	inferred, err := InferFromOpenAPISpec(context.Background(), "/files", InferenceRequest{}, openAPISpec)
	if err != nil {
		t.Fatalf("InferFromOpenAPISpec returned error: %v", err)
	}
	if inferred.Format != ResourceFormatAny {
		t.Fatalf("expected mixed payload types to infer format any, got %q", inferred.Format)
	}
}

func TestCompactInferredMetadataDefaultsPreservesFormat(t *testing.T) {
	t.Parallel()

	compact, err := CompactInferredMetadataDefaults("/admin/realms", ResourceMetadata{
		Format: "yaml",
	}, nil)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}
	if compact.Format != "yaml" {
		t.Fatalf("expected format yaml, got %#v", compact.Format)
	}
}

func TestCompactInferredMetadataDefaultsPreservesSecret(t *testing.T) {
	t.Parallel()

	secret := true
	compact, err := CompactInferredMetadataDefaults("/admin/realms", ResourceMetadata{
		Secret: &secret,
	}, nil)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}
	if compact.Secret == nil || !*compact.Secret {
		t.Fatalf("expected secret to be preserved as true, got %v", compact.Secret)
	}
}

func TestInferFromOpenAPIRejectsRecursiveRequest(t *testing.T) {
	t.Parallel()

	_, err := InferFromOpenAPISpec(
		context.Background(),
		"/admin/realms",
		InferenceRequest{Recursive: true},
		nil,
	)
	assertValidationError(t, err)
}

func TestHasOpenAPIPath(t *testing.T) {
	t.Parallel()

	openAPISpec := map[string]any{
		"paths": map[string]any{
			"/admin/realms": map[string]any{
				"get":  map[string]any{},
				"post": map[string]any{},
			},
			"/admin/realms/{realm}": map[string]any{
				"get":    map[string]any{},
				"put":    map[string]any{},
				"delete": map[string]any{},
			},
		},
	}

	exists, err := HasOpenAPIPath("/admin/realms/", openAPISpec)
	if err != nil {
		t.Fatalf("HasOpenAPIPath returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected /admin/realms/ to be found in OpenAPI paths")
	}

	exists, err = HasOpenAPIPath("/admin/realms/master", openAPISpec)
	if err != nil {
		t.Fatalf("HasOpenAPIPath returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected /admin/realms/master to be found in OpenAPI paths")
	}

	exists, err = HasOpenAPIPath("/admin/unknown/", openAPISpec)
	if err != nil {
		t.Fatalf("HasOpenAPIPath returned error: %v", err)
	}
	if exists {
		t.Fatal("expected /admin/unknown/ to be missing from OpenAPI paths")
	}
}

func TestValidateOperationSpecTemplatesAcceptsValidTemplate(t *testing.T) {
	t.Parallel()

	err := ValidateOperationSpecTemplates("get", OperationSpec{
		Method:  "GET",
		Path:    "./{{json_pointer \"/id\"}}",
		Accept:  "{{payload_media_type .}}",
		Headers: map[string]string{"X-Realm": "{{json_pointer \"/realm\"}}"},
	})
	if err != nil {
		t.Fatalf("expected valid templates to pass, got %v", err)
	}
}

func TestValidateOperationSpecTemplatesRejectsMalformedTemplate(t *testing.T) {
	t.Parallel()

	err := ValidateOperationSpecTemplates("get", OperationSpec{
		Path: "{{if .}}",
	})
	assertValidationError(t, err)
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != faults.ValidationError {
		t.Fatalf("expected %q category, got %q", faults.ValidationError, typedErr.Category)
	}
}
