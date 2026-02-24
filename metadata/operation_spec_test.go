package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestResolveOperationSpecMergesAndRenders(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveOperationSpec(context.Background(), ResourceMetadata{
		Filter:   []string{"/root"},
		Suppress: []string{"/internal"},
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:    "/api/customers/{{.id}}",
				Headers: map[string]string{"X-Tenant": "{{.tenant}}"},
				Query:   map[string]string{"expand": "{{.expand}}"},
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
	if len(resolved.Filter) != 1 || resolved.Filter[0] != "/root" {
		t.Fatalf("expected inherited filter, got %+v", resolved.Filter)
	}
	if len(resolved.Suppress) != 1 || resolved.Suppress[0] != "/internal" {
		t.Fatalf("expected inherited suppress, got %+v", resolved.Suppress)
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

func TestResolveOperationSpecWithScopeSupportsCollectionPathIndirection(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveOperationSpecWithScope(
		context.Background(),
		ResourceMetadata{
			CollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Path: "./{{.id}}",
				},
			},
		},
		OperationGet,
		map[string]any{
			"realm":          "platform",
			"id":             "123456",
			"collectionPath": "/admin/realms/platform/user-registry",
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
		CollectionPath: "/admin/realms/{{.realm}}/components",
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

func TestResolveOperationSpecWithScopeSupportsResourceFormatTemplateFunc(t *testing.T) {
	t.Parallel()

	md := ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:        "/api/customers/{{.id}}",
				Accept:      "application/{{resource_format .}}",
				ContentType: "application/{{resource_format .}}",
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
		if spec.Accept != "application/json" {
			t.Fatalf("expected accept application/json, got %q", spec.Accept)
		}
		if spec.ContentType != "application/json" {
			t.Fatalf("expected contentType application/json, got %q", spec.ContentType)
		}
	})

	t.Run("uses_yaml_when_provided", func(t *testing.T) {
		t.Parallel()

		spec, err := ResolveOperationSpecWithScope(context.Background(), md, OperationGet, map[string]any{
			"id":             "acme",
			"resourceFormat": "yaml",
		})
		if err != nil {
			t.Fatalf("ResolveOperationSpecWithScope returned error: %v", err)
		}
		if spec.Accept != "application/yaml" {
			t.Fatalf("expected accept application/yaml, got %q", spec.Accept)
		}
	})
}

func TestResolveOperationSpecWithScopeRejectsInvalidResourceFormatTemplateUsage(t *testing.T) {
	t.Parallel()

	_, err := ResolveOperationSpecWithScope(context.Background(), ResourceMetadata{
		Operations: map[string]OperationSpec{
			string(OperationGet): {
				Path:   "/api/customers/{{.id}}",
				Accept: "application/{{resource_format \"yaml\"}}",
			},
		},
	}, OperationGet, map[string]any{
		"id":             "acme",
		"resourceFormat": "yaml",
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "clientId" {
		t.Fatalf("expected aliasFromAttribute to be inferred as clientId, got %q", inferred.AliasFromAttribute)
	}
	if len(inferred.SecretsFromAttributes) != 1 || inferred.SecretsFromAttributes[0] != "secret" {
		t.Fatalf("expected inferred secret attribute [secret], got %#v", inferred.SecretsFromAttributes)
	}

	listOperation := inferred.Operations[string(OperationList)]
	if listOperation.Path != "/admin/realms/{{.realm}}/clients" {
		t.Fatalf("unexpected inferred list operation path: %+v", listOperation)
	}

	getOperation := inferred.Operations[string(OperationGet)]
	if getOperation.Path != "/admin/realms/{{.realm}}/clients/{{.clientId}}" {
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "alias" {
		t.Fatalf("expected aliasFromAttribute to be inferred as alias, got %q", inferred.AliasFromAttribute)
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "alias" {
		t.Fatalf("expected aliasFromAttribute to be inferred as alias, got %q", inferred.AliasFromAttribute)
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "alias" {
		t.Fatalf("expected aliasFromAttribute to be inferred as alias, got %q", inferred.AliasFromAttribute)
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "alias" {
		t.Fatalf("expected aliasFromAttribute to be inferred as alias, got %q", inferred.AliasFromAttribute)
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

	if inferred.IDFromAttribute != "realm" {
		t.Fatalf("expected idFromAttribute to be inferred as realm, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "realm" {
		t.Fatalf("expected aliasFromAttribute to be inferred as realm, got %q", inferred.AliasFromAttribute)
	}

	listOperation := inferred.Operations[string(OperationList)]
	if listOperation.Path != "/admin/realms" {
		t.Fatalf("unexpected inferred list operation path: %+v", listOperation)
	}

	getOperation := inferred.Operations[string(OperationGet)]
	if getOperation.Path != "/admin/realms/{{.realm}}" {
		t.Fatalf("unexpected inferred get operation path: %+v", getOperation)
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

	if compact.IDFromAttribute != "realm" {
		t.Fatalf("expected idFromAttribute to be preserved, got %q", compact.IDFromAttribute)
	}
	if compact.AliasFromAttribute != "realm" {
		t.Fatalf("expected aliasFromAttribute to be preserved, got %q", compact.AliasFromAttribute)
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

	if inferred.IDFromAttribute != "id" {
		t.Fatalf("expected idFromAttribute to be inferred as id, got %q", inferred.IDFromAttribute)
	}
	if inferred.AliasFromAttribute != "clientId" {
		t.Fatalf("expected aliasFromAttribute to be inferred as clientId, got %q", inferred.AliasFromAttribute)
	}
	if len(inferred.SecretsFromAttributes) != 1 || inferred.SecretsFromAttributes[0] != "secret" {
		t.Fatalf("expected inferred secret attribute [secret], got %#v", inferred.SecretsFromAttributes)
	}

	compact, err := CompactInferredMetadataDefaults("/admin/realms/_/clients/", inferred, openAPISpec)
	if err != nil {
		t.Fatalf("CompactInferredMetadataDefaults returned error: %v", err)
	}

	if len(compact.Operations) != 0 {
		t.Fatalf("expected openapi-default operations to be omitted, got %#v", compact.Operations)
	}
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
