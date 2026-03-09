package http

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/itchyny/gojq"

	"github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func jqMutation(expression string) []metadata.TransformStep {
	return []metadata.TransformStep{{JQExpression: expression}}
}

func suppressMutation(attributes ...string) []metadata.TransformStep {
	return []metadata.TransformStep{{ExcludeAttributes: attributes}}
}

func TestNewClientValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing_base_url", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("oauth2_grant_type_not_supported", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				OAuth2: &config.OAuth2{
					TokenURL:     "https://example.com/oauth/token",
					GrantType:    "password",
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("tls_client_pair_must_be_complete", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			TLS: &config.TLS{
				ClientCertFile: "/tmp/only-cert.pem",
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("openapi_http_url_not_allowed", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			OpenAPI: "http://example.com/openapi.json",
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("proxy_empty_block_disables_default", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			Proxy: &config.HTTPProxy{},
		})
		if err != nil {
			t.Fatalf("expected empty proxy block to be valid, got %v", err)
		}
	})

	t.Run("proxy_auth_rejects_embedded_credentials", func(t *testing.T) {
		t.Parallel()

		_, err := NewClient(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			Proxy: &config.HTTPProxy{
				HTTPURL: "http://user:pass@proxy.example.com:3128",
				Auth: &config.ProxyAuth{
					Username: "proxy-user",
					Password: "proxy-pass",
				},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestGetAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("fails_when_auth_is_not_oauth2", func(t *testing.T) {
		t.Parallel()

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token-123"}},
			},
		})

		_, err := client.GetAccessToken(context.Background())
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "oauth2") {
			t.Fatalf("expected oauth2 validation error, got %v", err)
		}
	})

	t.Run("returns_oauth2_access_token", func(t *testing.T) {
		t.Parallel()

		var tokenRequests int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth/token" {
				http.NotFound(w, r)
				return
			}
			atomic.AddInt32(&tokenRequests, 1)
			_, _ = fmt.Fprint(w, `{"access_token":"oauth-token","expires_in":3600}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				OAuth2: &config.OAuth2{
					TokenURL:     server.URL + "/oauth/token",
					GrantType:    config.OAuthClientCreds,
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
		})

		tokenOne, err := client.GetAccessToken(context.Background())
		if err != nil {
			t.Fatalf("GetAccessToken first call returned error: %v", err)
		}
		tokenTwo, err := client.GetAccessToken(context.Background())
		if err != nil {
			t.Fatalf("GetAccessToken second call returned error: %v", err)
		}
		if tokenOne != "oauth-token" || tokenTwo != "oauth-token" {
			t.Fatalf("expected oauth token, got %q and %q", tokenOne, tokenTwo)
		}
		if got := atomic.LoadInt32(&tokenRequests); got != 1 {
			t.Fatalf("expected one oauth token request, got %d", got)
		}
	})
}

func TestBuildRequestFromMetadataDefaultsAndHeaders(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		DefaultHeaders: map[string]string{
			"X-Default":  "base",
			"X-Override": "base",
		},
	})

	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationCreate): {
				Path:    "customers",
				Query:   map[string]string{"expand": "true"},
				Headers: map[string]string{"X-Override": "operation"},
			},
		},
	}
	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath: "/customers/acme",
		Payload:     map[string]any{"name": "ACME"},
	}, md, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	if spec.Method != http.MethodPost {
		t.Fatalf("expected POST method, got %q", spec.Method)
	}
	if spec.Path != "/customers" {
		t.Fatalf("expected path /customers, got %q", spec.Path)
	}
	if spec.Accept != defaultMediaType {
		t.Fatalf("expected default accept %q, got %q", defaultMediaType, spec.Accept)
	}
	if spec.ContentType != defaultMediaType {
		t.Fatalf("expected default content type %q, got %q", defaultMediaType, spec.ContentType)
	}
	if spec.Headers["X-Default"] != "base" {
		t.Fatalf("expected inherited default header, got %+v", spec.Headers)
	}
	if spec.Headers["X-Override"] != "operation" {
		t.Fatalf("expected operation header override, got %+v", spec.Headers)
	}
	if spec.Query["expand"] != "true" {
		t.Fatalf("expected query parameter to be preserved, got %+v", spec.Query)
	}
	if spec.Body == nil {
		t.Fatal("expected request body to default from resource payload")
	}
}

func TestBuildRequestFromMetadataDefaultsUseJSONWhenNoPayloadTypeIsConfigured(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})
	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationCreate): {Path: "/customers"},
		},
	}
	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath: "/customers/acme",
		Payload:     map[string]any{"name": "ACME"},
	}, md, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	if spec.Accept != defaultMediaType {
		t.Fatalf("expected default accept %q, got %q", defaultMediaType, spec.Accept)
	}
	if spec.ContentType != defaultMediaType {
		t.Fatalf("expected default content type %q, got %q", defaultMediaType, spec.ContentType)
	}
}

func TestBuildRequestFromMetadataHTTPMethodOverrideFromContext(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	ctx := metadata.WithOperationHTTPMethodOverride(context.Background(), metadata.OperationCreate, http.MethodPut)
	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationCreate): {
				Method: http.MethodPost,
				Path:   "/customers",
			},
		},
	}
	spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
		LogicalPath: "/customers/acme",
		Payload:     map[string]any{"name": "ACME"},
	}, md, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	if spec.Method != http.MethodPut {
		t.Fatalf("expected context override method PUT, got %q", spec.Method)
	}
}

func TestBuildRequestFromMetadataRendersTemplates(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationCreate): {
				Path: "/admin/realms/{{.realm}}/clients",
				Query: map[string]string{
					"clientId": "{{.clientId}}",
				},
			},
		},
	}
	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
		CollectionPath: "/admin/realms/platform/clients",
		Payload: map[string]any{
			"realm":    "platform",
			"clientId": "declarest-cli",
		},
	}, md, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	if spec.Path != "/admin/realms/platform/clients" {
		t.Fatalf("expected rendered path /admin/realms/platform/clients, got %q", spec.Path)
	}
	if spec.Query["clientId"] != "declarest-cli" {
		t.Fatalf("expected rendered query clientId=declarest-cli, got %+v", spec.Query)
	}
}

func TestBuildRequestFromMetadataListUsesRenderedCollectionPathTemplate(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	md := metadata.ResourceMetadata{
		IDAttribute:          "/id",
		AliasAttribute:       "/name",
		RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationList): {
				Transforms: jqMutation(`[ .[] | select(.providerId == "ldap") ]`),
			},
		},
	}
	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath:    "/admin/realms/publico-br/user-registry",
		CollectionPath: "/admin/realms/publico-br",
	}, md, metadata.OperationList)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	if spec.Path != "/admin/realms/publico-br/components" {
		t.Fatalf("expected rendered list path /admin/realms/publico-br/components, got %q", spec.Path)
	}
	if !reflect.DeepEqual(spec.Transforms, jqMutation(`[ .[] | select(.providerId == "ldap") ]`)) {
		t.Fatalf("expected list transforms to be preserved, got %#v", spec.Transforms)
	}
}

func TestBuildRequestFromMetadataAppliesPayloadTransformsForCreateUpdate(t *testing.T) {
	t.Parallel()

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationCreate): {
				Path: "/bla",
				Transforms: []metadata.TransformStep{
					{ExcludeAttributes: []string{"/bla"}},
					{JQExpression: ". | .c = .test"},
				},
			},
		},
	}
	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath: "/bla/ble",
		Payload: map[string]any{
			"a":    "b",
			"bla":  "ble",
			"test": "xxx",
		},
	}, md, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	bodyContent, ok := spec.Body.(resource.Content)
	if !ok {
		t.Fatalf("expected transformed request body content, got %T", spec.Body)
	}
	body, ok := bodyContent.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected transformed request body object, got %T", bodyContent.Value)
	}
	if _, exists := body["bla"]; exists {
		t.Fatalf("expected excludeAttributes to remove bla, got %#v", body)
	}
	if body["a"] != "b" || body["test"] != "xxx" || body["c"] != "xxx" {
		t.Fatalf("unexpected transformed request body %#v", body)
	}
}

func TestBuildRequestFromMetadataAppliesPayloadTransformsInMetadataTransformsOrder(t *testing.T) {
	t.Parallel()

	decodedMetadata, err := metadata.DecodeResourceMetadataJSON([]byte(`{
	  "operations": {
	    "create": {
	      "path": "/bla",
	      "transforms": [
	        {"jqExpression": ". | .c = .bla"},
	        {"excludeAttributes": ["/bla"]}
	      ]
	    }
	  }
	}`))
	if err != nil {
		t.Fatalf("DecodeResourceMetadataJSON returned error: %v", err)
	}

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath: "/bla/ble",
		Payload: map[string]any{
			"a":   "b",
			"bla": "ble",
		},
	}, decodedMetadata, metadata.OperationCreate)
	if err != nil {
		t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
	}

	bodyContent, ok := spec.Body.(resource.Content)
	if !ok {
		t.Fatalf("expected transformed request body content, got %T", spec.Body)
	}
	body, ok := bodyContent.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected transformed request body object, got %T", bodyContent.Value)
	}
	if _, exists := body["bla"]; exists {
		t.Fatalf("expected excludeAttributes to remove bla, got %#v", body)
	}
	if body["c"] != "ble" {
		t.Fatalf("expected jqExpression to run before excludeAttributes based on metadata field order, got %#v", body)
	}
}

func TestBuildRequestFromMetadataRundeckFixtureSelectors(t *testing.T) {
	t.Parallel()

	metadataDir := filepath.Join(
		"..",
		"..",
		"..",
		"..",
		"test",
		"e2e",
		"components",
		"managed-server",
		"rundeck",
		"metadata",
	)
	service := fsmetadata.NewFSMetadataService(metadataDir)
	client := mustManagedServerClient(
		t,
		config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "token",
				}},
			},
		},
		WithMetadataRenderer(service),
	)
	ctx := context.Background()

	t.Run("project_get_normalizes_config_payload", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform",
			Payload: map[string]any{
				"name":        "platform",
				"description": "Managed by declarest E2E",
				"config": map[string]any{
					"project.label":       "Platform",
					"project.description": "Managed by declarest E2E",
				},
			},
		}, md, metadata.OperationGet)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/project/platform/config" {
			t.Fatalf("expected project config path, got %q", spec.Path)
		}

		value, err := client.applyOperationPayloadTransforms(ctx, map[string]any{
			"project.label":       "Platform",
			"project.description": "Managed by declarest E2E",
		}, spec)
		if err != nil {
			t.Fatalf("applyOperationPayloadTransforms returned error: %v", err)
		}

		expected := map[string]any{
			"name":        "platform",
			"description": "Managed by declarest E2E",
			"config": map[string]any{
				"project.name":        "platform",
				"project.label":       "Platform",
				"project.description": "Managed by declarest E2E",
			},
		}
		if !reflect.DeepEqual(expected, value) {
			t.Fatalf("unexpected normalized project payload %#v", value)
		}
	})

	t.Run("project_update_uses_config_body", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform",
			Payload: map[string]any{
				"name":        "platform",
				"description": "Managed by declarest E2E [rev-2]",
				"config": map[string]any{
					"project.label":       "Platform",
					"project.description": "Managed by declarest E2E",
				},
			},
		}, md, metadata.OperationUpdate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Method != http.MethodPut {
			t.Fatalf("expected PUT method, got %q", spec.Method)
		}
		if spec.Path != "/project/platform/config" {
			t.Fatalf("expected project config path, got %q", spec.Path)
		}

		bodyContent, ok := spec.Body.(resource.Content)
		if !ok {
			t.Fatalf("expected project config body content, got %T", spec.Body)
		}
		body, ok := bodyContent.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected project config body object, got %T", bodyContent.Value)
		}
		if body["project.description"] != "Managed by declarest E2E [rev-2]" {
			t.Fatalf("expected top-level description to flow into project config, got %#v", body)
		}
	})

	t.Run("job_create_uses_project_import_path", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/jobs/sync-platform")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform/jobs/sync-platform",
			Payload: map[string]any{
				"id":          "22222222-2222-4222-8222-222222222222",
				"name":        "sync-platform",
				"project":     "platform",
				"description": "Synchronize platform configuration",
				"sequence": map[string]any{
					"keepgoing": false,
					"strategy":  "node-first",
					"commands": []any{
						map[string]any{"exec": "echo sync-platform"},
					},
				},
			},
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/project/platform/jobs/import" {
			t.Fatalf("expected project jobs import path, got %q", spec.Path)
		}
		if spec.Query["dupeOption"] != "create" || spec.Query["uuidOption"] != "preserve" {
			t.Fatalf("unexpected job import query %#v", spec.Query)
		}
		bodyContent, ok := spec.Body.(resource.Content)
		if !ok {
			t.Fatalf("expected job import body content, got %T", spec.Body)
		}
		body, ok := bodyContent.Value.([]any)
		if !ok || len(body) != 1 {
			t.Fatalf("expected single-item job import array, got %#v", bodyContent.Value)
		}
		job, ok := body[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first import item to be an object, got %#v", body[0])
		}
		if job["uuid"] != "22222222-2222-4222-8222-222222222222" {
			t.Fatalf("expected import payload to map repo id to uuid, got %#v", job)
		}
		if _, hasConfig := job["config"]; hasConfig {
			t.Fatalf("expected job import payload not to inherit project config transform, got %#v", job)
		}
	})

	t.Run("job_get_uses_uuid_path_and_normalizes_exported_payload", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/jobs/sync-platform")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform/jobs/sync-platform",
			Payload: map[string]any{
				"id":          "22222222-2222-4222-8222-222222222222",
				"name":        "sync-platform",
				"project":     "platform",
				"description": "Synchronize platform configuration",
			},
		}, md, metadata.OperationGet)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/job/22222222-2222-4222-8222-222222222222" {
			t.Fatalf("expected job uuid path, got %q", spec.Path)
		}

		value, err := client.applyOperationPayloadTransforms(ctx, []any{
			map[string]any{
				"id":          "22222222-2222-4222-8222-222222222222",
				"uuid":        "22222222-2222-4222-8222-222222222222",
				"name":        "sync-platform",
				"description": "Synchronize platform configuration",
				"sequence": map[string]any{
					"keepgoing": false,
					"strategy":  "node-first",
					"commands": []any{
						map[string]any{"exec": "echo sync-platform"},
					},
				},
			},
		}, spec)
		if err != nil {
			t.Fatalf("applyOperationPayloadTransforms returned error: %v", err)
		}

		expected := map[string]any{
			"id":          "22222222-2222-4222-8222-222222222222",
			"name":        "sync-platform",
			"project":     "platform",
			"description": "Synchronize platform configuration",
			"sequence": map[string]any{
				"keepgoing": false,
				"strategy":  "node-first",
				"commands": []any{
					map[string]any{"exec": "echo sync-platform"},
				},
			},
		}
		if !reflect.DeepEqual(expected, value) {
			t.Fatalf("unexpected normalized job payload %#v", value)
		}
	})

	t.Run("nodes_list_uses_project_sources_path", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/nodes")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath:    "/projects/platform/nodes",
			CollectionPath: "/projects/platform/nodes",
		}, md, metadata.OperationList)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/project/platform/sources" {
			t.Fatalf("expected project sources list path, got %q", spec.Path)
		}
		if !reflect.DeepEqual(
			spec.Transforms,
			jqMutation(`map(. + {index: (.index | tostring), project: "platform"})`),
		) {
			t.Fatalf("unexpected rendered nodes transforms %#v", spec.Transforms)
		}
	})

	t.Run("secret_update_uses_project_key_storage", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/secrets/db-password")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform/secrets/db-password",
			Payload: map[string]any{
				"name":    "db-password",
				"type":    "password",
				"content": "super-secret",
			},
		}, md, metadata.OperationUpdate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/storage/keys/project/platform/db-password" {
			t.Fatalf("expected project key-storage path, got %q", spec.Path)
		}
		if spec.ContentType != "application/x-rundeck-data-password" {
			t.Fatalf("expected rundeck password content type, got %q", spec.ContentType)
		}
		bodyContent, ok := spec.Body.(resource.Content)
		if !ok {
			t.Fatalf("expected secret body content, got %T", spec.Body)
		}
		if body, ok := bodyContent.Value.(string); !ok || body != "super-secret" {
			t.Fatalf("expected secret content body, got %#v", bodyContent.Value)
		}
	})

	t.Run("secret_update_accepts_raw_private_key_payload", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/secrets/private-key")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform/secrets/private-key",
			Payload:     resource.BinaryValue{Bytes: []byte("private-key-bytes")},
			PayloadDescriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
				Extension: ".key",
			}),
		}, md, metadata.OperationUpdate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/storage/keys/project/platform/private-key" {
			t.Fatalf("expected project key-storage path, got %q", spec.Path)
		}
		if spec.Accept != defaultMediaType {
			t.Fatalf("expected metadata accept %q, got %q", defaultMediaType, spec.Accept)
		}
		if spec.ContentType != "application/octet-stream" {
			t.Fatalf("expected octet-stream content type, got %q", spec.ContentType)
		}
		bodyContent, ok := spec.Body.(resource.Content)
		if !ok {
			t.Fatalf("expected secret body content, got %T", spec.Body)
		}
		body, ok := bodyContent.Value.(resource.BinaryValue)
		if !ok {
			t.Fatalf("expected binary secret body, got %T", bodyContent.Value)
		}
		if string(body.Bytes) != "private-key-bytes" {
			t.Fatalf("expected raw binary secret body, got %q", string(body.Bytes))
		}
		if bodyContent.Descriptor.Extension != ".key" {
			t.Fatalf("expected .key descriptor extension, got %q", bodyContent.Descriptor.Extension)
		}
	})

	t.Run("secret_get_requests_metadata_for_raw_private_key_payloads", func(t *testing.T) {
		t.Parallel()

		md, err := service.ResolveForPath(ctx, "/projects/platform/secrets/private-key")
		if err != nil {
			t.Fatalf("ResolveForPath returned error: %v", err)
		}

		spec, err := client.BuildRequestFromMetadata(ctx, resource.Resource{
			LogicalPath: "/projects/platform/secrets/private-key",
			Payload:     resource.BinaryValue{Bytes: []byte("private-key-bytes")},
			PayloadDescriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
				Extension: ".key",
			}),
		}, md, metadata.OperationGet)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.Path != "/storage/keys/project/platform/private-key" {
			t.Fatalf("expected project key-storage path, got %q", spec.Path)
		}
		if spec.Accept != defaultMediaType {
			t.Fatalf("expected metadata accept %q, got %q", defaultMediaType, spec.Accept)
		}
		if spec.Body != nil {
			t.Fatalf("expected GET request to have no body, got %#v", spec.Body)
		}
	})
}

func TestBuildRequestFromMetadataValidatesOperationPayloadRules(t *testing.T) {
	t.Parallel()

	t.Run("required_attributes_accept_path_derived_fields", func(t *testing.T) {
		t.Parallel()

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		md := metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{.realm}}/clients",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/admin/realms/{{.realm}}/clients",
					Validate: &metadata.OperationValidationSpec{
						RequiredAttributes: []string{"/realm", "/clientId"},
					},
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
			CollectionPath: "/admin/realms/platform/clients",
			Payload: map[string]any{
				"clientId": "declarest-cli",
			},
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}
	})

	t.Run("required_attributes_fail_when_missing", func(t *testing.T) {
		t.Parallel()

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/customers",
					Validate: &metadata.OperationValidationSpec{
						RequiredAttributes: []string{"/realm"},
					},
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Payload: map[string]any{
				"name": "Acme",
			},
		}, md, metadata.OperationCreate)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "missing required attributes") {
			t.Fatalf("expected missing required attributes validation error, got %v", err)
		}
	})

	t.Run("assertion_failure_uses_assertion_message", func(t *testing.T) {
		t.Parallel()

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/customers",
					Validate: &metadata.OperationValidationSpec{
						Assertions: []metadata.ValidationAssertion{
							{
								Message: "realm is mandatory to create a resource.",
								JQ:      `has("realm") and (.realm | type=="string") and (.realm|length>0)`,
							},
						},
					},
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Payload: map[string]any{
				"name": "Acme",
			},
		}, md, metadata.OperationCreate)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "realm is mandatory to create a resource.") {
			t.Fatalf("expected assertion message in validation error, got %v", err)
		}
	})

	t.Run("schema_ref_request_body_uses_path_fields_and_required_properties", func(t *testing.T) {
		t.Parallel()

		openAPI := `
openapi: 3.0.0
paths:
  /admin/realms/{realm}/clients:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required:
                - realm
                - clientId
              properties:
                realm:
                  type: string
                clientId:
                  type: string
      responses:
        "201":
          content:
            application/json: {}
`
		tempDir := t.TempDir()
		specPath := filepath.Join(tempDir, "openapi.yaml")
		if err := os.WriteFile(specPath, []byte(openAPI), 0o600); err != nil {
			t.Fatalf("failed to write openapi fixture: %v", err)
		}

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			OpenAPI: specPath,
		})

		md := metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{.realm}}/clients",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/admin/realms/{{.realm}}/clients",
					Validate: &metadata.OperationValidationSpec{
						SchemaRef: "openapi:request-body",
					},
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
			CollectionPath: "/admin/realms/platform/clients",
			Payload: map[string]any{
				"clientId": "declarest-cli",
			},
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		md2 := metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{.realm}}/clients",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/admin/realms/{{.realm}}/clients",
					Validate: &metadata.OperationValidationSpec{
						SchemaRef: "openapi:request-body",
					},
				},
			},
		}
		_, err = client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
			CollectionPath: "/admin/realms/platform/clients",
			Payload:        map[string]any{},
		}, md2, metadata.OperationCreate)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "missing required property") {
			t.Fatalf("expected schema required-property validation error, got %v", err)
		}
	})

	t.Run("schema_ref_request_body_supports_swagger2", func(t *testing.T) {
		t.Parallel()

		openAPI := `
swagger: "2.0"
consumes:
  - application/json
produces:
  - application/json
paths:
  /admin/realms/{realm}/clients:
    post:
      parameters:
        - name: realm
          in: path
          required: true
          type: string
        - name: payload
          in: body
          required: true
          schema:
            type: object
            required:
              - realm
              - clientId
            properties:
              realm:
                type: string
              clientId:
                type: string
      responses:
        "201":
          description: created
          schema:
            type: object
            properties:
              clientId:
                type: string
`
		tempDir := t.TempDir()
		specPath := filepath.Join(tempDir, "swagger.yaml")
		if err := os.WriteFile(specPath, []byte(openAPI), 0o600); err != nil {
			t.Fatalf("failed to write swagger fixture: %v", err)
		}

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			OpenAPI: specPath,
		})

		md := metadata.ResourceMetadata{
			RemoteCollectionPath: "/admin/realms/{{.realm}}/clients",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/admin/realms/{{.realm}}/clients",
					Validate: &metadata.OperationValidationSpec{
						SchemaRef: "openapi:request-body",
					},
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
			CollectionPath: "/admin/realms/platform/clients",
			Payload: map[string]any{
				"clientId": "declarest-cli",
			},
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		_, err = client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
			CollectionPath: "/admin/realms/platform/clients",
			Payload:        map[string]any{},
		}, md, metadata.OperationCreate)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "missing required property") {
			t.Fatalf("expected schema required-property validation error, got %v", err)
		}
	})
}

func TestRequestAppliesMetadataValidationFromContext(t *testing.T) {
	t.Parallel()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	validationCtx := metadata.WithRequestOperationValidation(
		context.Background(),
		metadata.OperationCreate,
		metadata.ResourceOperationSpecInput{
			LogicalPath:    "/admin/realms/platform/clients",
			CollectionPath: "/admin/realms/platform/clients",
			Metadata: metadata.ResourceMetadata{
				RemoteCollectionPath: "/admin/realms/{{.realm}}/clients",
			},
		},
		&metadata.OperationValidationSpec{
			RequiredAttributes: []string{"/realm", "/clientId"},
		},
	)

	_, err := client.Request(
		validationCtx,
		managedserverdomain.RequestSpec{
			Method: http.MethodPost,
			Path:   "/admin/realms/platform/clients",
			Body: resource.Content{
				Value: map[string]any{"clientId": "declarest-cli"},
			},
		},
	)
	if err != nil {
		t.Fatalf("Request returned error: %v", err)
	}

	_, err = client.Request(
		validationCtx,
		managedserverdomain.RequestSpec{
			Method: http.MethodPost,
			Path:   "/admin/realms/platform/clients",
			Body: resource.Content{
				Value: map[string]any{},
			},
		},
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("expected one successful remote request after validation short-circuit, got %d", got)
	}
}

func TestGetAppliesPayloadTransformsAfterResponseDecode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/bla/ble" {
			t.Fatalf("expected /bla/ble path, got %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"a":"b","bla":"ble","test":"xxx"}`)
	}))
	t.Cleanup(server.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
	})

	md := metadata.ResourceMetadata{
		Operations: map[string]metadata.OperationSpec{
			string(metadata.OperationGet): {
				Path: "/bla/ble",
				Transforms: []metadata.TransformStep{
					{ExcludeAttributes: []string{"/bla"}},
					{JQExpression: ". | .c = .test"},
				},
			},
		},
	}
	value, err := client.Get(context.Background(), resource.Resource{
		LogicalPath: "/bla/ble",
	}, md)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	objectValue, ok := value.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected transformed get payload object, got %T", value)
	}
	if _, exists := objectValue["bla"]; exists {
		t.Fatalf("expected excludeAttributes to remove bla, got %#v", objectValue)
	}
	if objectValue["a"] != "b" || objectValue["test"] != "xxx" || objectValue["c"] != "xxx" {
		t.Fatalf("unexpected transformed get payload %#v", objectValue)
	}
}

func TestOpenAPIFallbackAndValidation(t *testing.T) {
	t.Parallel()

	openAPI := `
openapi: 3.0.0
paths:
  /customers:
    post:
      requestBody:
        content:
          application/xml: {}
      responses:
        "201":
          content:
            application/hal+json: {}
    get:
      responses:
        "200":
          content:
            application/problem+json: {}
`

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(openAPI), 0o600); err != nil {
		t.Fatalf("failed to write openapi fixture: %v", err)
	}

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		OpenAPI: specPath,
	})

	t.Run("fills_missing_fields_from_openapi", func(t *testing.T) {
		t.Parallel()

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/customers",
				},
			},
		}
		spec, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.ContentType != "application/xml" {
			t.Fatalf("expected content type from openapi, got %q", spec.ContentType)
		}
		if spec.Accept != "application/hal+json" {
			t.Fatalf("expected accept from openapi, got %q", spec.Accept)
		}
	})

	t.Run("fails_when_openapi_path_does_not_support_method", func(t *testing.T) {
		t.Parallel()

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationDelete): {
					Path: "/customers",
				},
			},
		}
		_, err := client.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md, metadata.OperationDelete)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("fills_missing_fields_from_swagger2", func(t *testing.T) {
		t.Parallel()

		swagger := `
swagger: "2.0"
consumes:
  - application/xml
produces:
  - application/hal+json
paths:
  /customers:
    post:
      parameters:
        - name: payload
          in: body
          schema:
            type: object
      responses:
        "201":
          description: created
          schema:
            type: object
`
		tempDir := t.TempDir()
		specPath := filepath.Join(tempDir, "swagger.yaml")
		if err := os.WriteFile(specPath, []byte(swagger), 0o600); err != nil {
			t.Fatalf("failed to write swagger fixture: %v", err)
		}

		swaggerClient := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com/api",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			OpenAPI: specPath,
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/customers",
				},
			},
		}
		spec, err := swaggerClient.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md, metadata.OperationCreate)
		if err != nil {
			t.Fatalf("BuildRequestFromMetadata returned error: %v", err)
		}

		if spec.ContentType != "application/xml" {
			t.Fatalf("expected content type from swagger2 consumes, got %q", spec.ContentType)
		}
		if spec.Accept != "application/hal+json" {
			t.Fatalf("expected accept from swagger2 produces, got %q", spec.Accept)
		}
	})
}

func TestGetOpenAPISpecFromHTTPSCachesDocument(t *testing.T) {
	t.Parallel()

	var openAPICalls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			atomic.AddInt32(&openAPICalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"openapi":"3.0.0","paths":{"/resource":{"get":{"responses":{"200":{"content":{"application/json":{}}}}}}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		TLS: &config.TLS{
			InsecureSkipVerify: true,
		},
		OpenAPI: server.URL + "/openapi.json",
	})

	valueOne, err := client.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("GetOpenAPISpec first call returned error: %v", err)
	}
	valueTwo, err := client.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("GetOpenAPISpec second call returned error: %v", err)
	}

	if valueOne.Value == nil || valueTwo.Value == nil {
		t.Fatal("expected non-nil OpenAPI payload")
	}
	if got := atomic.LoadInt32(&openAPICalls); got != 1 {
		t.Fatalf("expected one openapi fetch, got %d", got)
	}
}

func TestGetOpenAPISpecFromHTTPSAllowsCrossOriginOpenAPIURLWithoutAuthHeader(t *testing.T) {
	t.Parallel()

	baseServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(baseServer.Close)

	var openAPICalls int32
	openAPIServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&openAPICalls, 1)
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected no auth header on cross-origin openapi request, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"openapi":"3.0.0","paths":{}}`)
	}))
	t.Cleanup(openAPIServer.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: baseServer.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		TLS: &config.TLS{
			InsecureSkipVerify: true,
		},
		OpenAPI: openAPIServer.URL + "/openapi.json",
	})

	value, err := client.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("expected cross-origin openapi request to succeed, got error: %v", err)
	}
	if value.Value == nil {
		t.Fatal("expected non-nil OpenAPI payload")
	}
	if got := atomic.LoadInt32(&openAPICalls); got != 1 {
		t.Fatalf("expected one cross-origin openapi request, got %d calls", got)
	}
}

func TestGetOpenAPISpecFromHTTPSSameOriginAppliesAuthHeader(t *testing.T) {
	t.Parallel()

	var openAPICalls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&openAPICalls, 1)
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("expected auth header on same-origin openapi request, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"openapi":"3.0.0","paths":{}}`)
	}))
	t.Cleanup(server.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		TLS: &config.TLS{
			InsecureSkipVerify: true,
		},
		OpenAPI: server.URL + "/openapi.json",
	})

	value, err := client.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("expected same-origin openapi request to succeed, got error: %v", err)
	}
	if value.Value == nil {
		t.Fatal("expected non-nil OpenAPI payload")
	}
	if got := atomic.LoadInt32(&openAPICalls); got != 1 {
		t.Fatalf("expected one same-origin openapi request, got %d calls", got)
	}
}

func TestGetOpenAPISpecFromHTTPSDoesNotCacheError(t *testing.T) {
	t.Parallel()

	var openAPICalls int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" {
			http.NotFound(w, r)
			return
		}
		call := atomic.AddInt32(&openAPICalls, 1)
		if call == 1 {
			http.Error(w, "temporary error", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"openapi":"3.0.0","paths":{"/resource":{"get":{"responses":{"200":{"content":{"application/json":{}}}}}}}}`)
	}))
	t.Cleanup(server.Close)

	client := mustManagedServerClient(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
		},
		TLS: &config.TLS{
			InsecureSkipVerify: true,
		},
		OpenAPI: server.URL + "/openapi.json",
	})

	_, err := client.GetOpenAPISpec(context.Background())
	assertTypedCategory(t, err, faults.TransportError)

	value, err := client.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("GetOpenAPISpec second call returned error: %v", err)
	}
	if value.Value == nil {
		t.Fatal("expected successful OpenAPI payload after retry")
	}
	if got := atomic.LoadInt32(&openAPICalls); got != 2 {
		t.Fatalf("expected two openapi fetches after transient failure, got %d", got)
	}
}

func TestAuthModesAndOAuth2Caching(t *testing.T) {
	t.Parallel()

	t.Run("basic_auth", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
			if got := r.Header.Get("Authorization"); got != expected {
				t.Fatalf("expected basic auth header %q, got %q", expected, got)
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BasicAuth: &config.BasicAuth{Username: "user", Password: "pass"},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	t.Run("bearer_auth", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("expected bearer header, got %q", got)
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token-123"}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	t.Run("custom_header_auth", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-Auth-Token"); got != "custom-token" {
				t.Fatalf("expected custom header token, got %q", got)
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "X-Auth-Token", Value: "custom-token"}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	t.Run("custom_header_auth_with_prefix", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer custom-token" {
				t.Fatalf("expected custom header with prefix, got %q", got)
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{
					Header: "Authorization",
					Prefix: "Bearer",
					Value:  "custom-token",
				}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
	})

	t.Run("oauth2_client_credentials_cached", func(t *testing.T) {
		t.Parallel()

		var tokenRequests int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				atomic.AddInt32(&tokenRequests, 1)
				_, _ = fmt.Fprint(w, `{"access_token":"oauth-token","expires_in":3600}`)
			case "/resource":
				if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
					t.Fatalf("expected bearer oauth token, got %q", got)
				}
				_, _ = fmt.Fprint(w, `{"ok":true}`)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				OAuth2: &config.OAuth2{
					TokenURL:     server.URL + "/oauth/token",
					GrantType:    config.OAuthClientCreds,
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		resource := resource.Resource{
			LogicalPath: "/customers/acme",
		}
		if _, err := client.Get(context.Background(), resource, md); err != nil {
			t.Fatalf("first Get returned error: %v", err)
		}
		if _, err := client.Get(context.Background(), resource, md); err != nil {
			t.Fatalf("second Get returned error: %v", err)
		}
		if got := atomic.LoadInt32(&tokenRequests); got != 1 {
			t.Fatalf("expected one oauth token request, got %d", got)
		}
	})

	t.Run("oauth2_debug_logs_include_token_request", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/oauth/token":
				_, _ = fmt.Fprint(w, `{"access_token":"oauth-token","expires_in":3600}`)
			case "/resource":
				_, _ = fmt.Fprint(w, `{"ok":true}`)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				OAuth2: &config.OAuth2{
					TokenURL:     server.URL + "/oauth/token",
					GrantType:    config.OAuthClientCreds,
					ClientID:     "client",
					ClientSecret: "secret-value",
				},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		resource := resource.Resource{
			LogicalPath: "/customers/acme",
		}

		var debugOutput bytes.Buffer
		ctx := debugctx.WithEnabled(context.Background(), true)
		ctx = debugctx.WithWriter(ctx, &debugOutput)

		if _, err := client.Get(ctx, resource, md); err != nil {
			t.Fatalf("Get returned error: %v", err)
		}

		contents := debugOutput.String()
		if !strings.Contains(contents, `purpose="oauth2-token"`) {
			t.Fatalf("expected oauth2 token request in debug output, got %q", contents)
		}
		if !strings.Contains(contents, "/oauth/token") {
			t.Fatalf("expected oauth2 token URL in debug output, got %q", contents)
		}
		if !strings.Contains(contents, `purpose="resource"`) {
			t.Fatalf("expected resource request in debug output, got %q", contents)
		}
		if !strings.Contains(contents, `tls_enabled=false`) {
			t.Fatalf("expected tls debug flag in debug output, got %q", contents)
		}
		if !strings.Contains(contents, `mtls_enabled=false`) {
			t.Fatalf("expected mtls debug flag in debug output, got %q", contents)
		}
		if strings.Contains(contents, "secret-value") {
			t.Fatalf("debug output leaked client secret: %q", contents)
		}
	})

	t.Run("debug_logs_include_mtls_configuration", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		caCertFile, clientCertFile, clientKeyFile := writeTLSClientPairFiles(t)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
			TLS: &config.TLS{
				CACertFile:         caCertFile,
				ClientCertFile:     clientCertFile,
				ClientKeyFile:      clientKeyFile,
				InsecureSkipVerify: true,
			},
		})

		var debugOutput bytes.Buffer
		ctx := debugctx.WithEnabled(context.Background(), true)
		ctx = debugctx.WithWriter(ctx, &debugOutput)

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(ctx, resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}

		contents := debugOutput.String()
		if !strings.Contains(contents, `tls_enabled=true`) {
			t.Fatalf("expected tls enabled in debug output, got %q", contents)
		}
		if !strings.Contains(contents, `mtls_enabled=true`) {
			t.Fatalf("expected mtls enabled in debug output, got %q", contents)
		}
		if !strings.Contains(contents, `tls_insecure_skip_verify=true`) {
			t.Fatalf("expected tls insecure skip verify in debug output, got %q", contents)
		}
		if !strings.Contains(contents, fmt.Sprintf(`tls_ca_cert_file=%q`, caCertFile)) {
			t.Fatalf("expected tls ca cert file in debug output, got %q", contents)
		}
		if !strings.Contains(contents, fmt.Sprintf(`tls_client_cert_file=%q`, clientCertFile)) {
			t.Fatalf("expected tls client cert file in debug output, got %q", contents)
		}
		if !strings.Contains(contents, fmt.Sprintf(`tls_client_key_file=%q`, clientKeyFile)) {
			t.Fatalf("expected tls client key file in debug output, got %q", contents)
		}
	})

	t.Run("debug_logs_redact_query_values", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		var debugOutput bytes.Buffer
		ctx := debugctx.WithEnabled(context.Background(), true)
		ctx = debugctx.WithWriter(ctx, &debugOutput)

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {
					Path:  "/search",
					Query: map[string]string{"token": "secret-query", "name": "alice"},
				},
			},
		}
		_, err := client.Get(ctx, resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}

		contents := debugOutput.String()
		if strings.Contains(contents, "secret-query") {
			t.Fatalf("debug output leaked query secret: %q", contents)
		}
		if !strings.Contains(contents, "token=%3Credacted%3E") {
			t.Fatalf("expected redacted token query value in debug output, got %q", contents)
		}
	})
}

func TestManagedServerProxySupport(t *testing.T) {
	t.Parallel()

	t.Run("resource_and_oauth2_requests_use_configured_proxy", func(t *testing.T) {
		t.Parallel()

		var proxyRequests int32
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&proxyRequests, 1)

			switch r.URL.Path {
			case "/oauth/token":
				if got := r.Header.Get("Proxy-Authorization"); got == "" {
					t.Fatalf("expected proxy authorization header, got empty value")
				}
				_, _ = fmt.Fprint(w, `{"access_token":"proxy-oauth-token","expires_in":3600}`)
			case "/resource":
				if got := r.Header.Get("Authorization"); got != "Bearer proxy-oauth-token" {
					t.Fatalf("expected oauth2 bearer header via proxy, got %q", got)
				}
				_, _ = fmt.Fprint(w, `{"ok":true}`)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(proxy.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "http://api.example.com",
			Auth: &config.HTTPAuth{
				OAuth2: &config.OAuth2{
					TokenURL:     "http://auth.example.com/oauth/token",
					GrantType:    config.OAuthClientCreds,
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
			Proxy: &config.HTTPProxy{
				HTTPURL: proxy.URL,
				Auth: &config.ProxyAuth{
					Username: "proxy-user",
					Password: "proxy-pass",
				},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		_, err := client.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}

		if got := atomic.LoadInt32(&proxyRequests); got != 2 {
			t.Fatalf("expected two proxy requests (oauth2 + resource), got %d", got)
		}
	})

	t.Run("no_proxy_bypasses_proxy_for_matching_host", func(t *testing.T) {
		t.Parallel()

		proxyFunc, err := buildProxyFunc(&config.HTTPProxy{
			HTTPURL: "http://proxy.example.com:3128",
			NoProxy: "api.example.com",
		})
		if err != nil {
			t.Fatalf("buildProxyFunc returned error: %v", err)
		}

		request, err := http.NewRequest(http.MethodGet, "http://api.example.com/resource", nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}

		resolvedProxy, err := proxyFunc(request)
		if err != nil {
			t.Fatalf("proxy resolver returned error: %v", err)
		}
		if resolvedProxy != nil {
			t.Fatalf("expected no-proxy match to bypass proxy, got %q", resolvedProxy.String())
		}

		otherRequest, err := http.NewRequest(http.MethodGet, "http://other.example.com/resource", nil)
		if err != nil {
			t.Fatalf("failed to build secondary request: %v", err)
		}
		resolvedProxy, err = proxyFunc(otherRequest)
		if err != nil {
			t.Fatalf("proxy resolver returned error for secondary request: %v", err)
		}
		if resolvedProxy == nil {
			t.Fatal("expected non-matching host to use configured proxy")
		}
		if resolvedProxy.String() != "http://proxy.example.com:3128" {
			t.Fatalf("expected configured proxy URL, got %q", resolvedProxy.String())
		}
	})
}

func TestRequestOperations(t *testing.T) {
	t.Parallel()

	t.Run("get_json_response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET method, got %s", r.Method)
			}
			if r.URL.Path != "/test" {
				t.Fatalf("expected /test path, got %s", r.URL.Path)
			}
			_, _ = fmt.Fprint(w, `{"id":"a"}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		value, err := client.Request(context.Background(), managedserverdomain.RequestSpec{
			Method: http.MethodGet,
			Path:   "/test",
		})
		if err != nil {
			t.Fatalf("Request returned error: %v", err)
		}

		output, ok := value.Value.(map[string]any)
		if !ok {
			t.Fatalf("expected map response, got %T", value)
		}
		if output["id"] != "a" {
			t.Fatalf("expected id=a response, got %#v", output)
		}
	})

	t.Run("post_json_body", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST method, got %s", r.Method)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected content type application/json, got %q", got)
			}
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if string(data) != `{"id":"a","name":"alpha"}` {
				t.Fatalf("unexpected request body: %s", string(data))
			}
			_, _ = fmt.Fprint(w, `{"ok":true}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.Request(context.Background(), managedserverdomain.RequestSpec{
			Method: http.MethodPost,
			Path:   "/test",
			Body: resource.Content{
				Value: map[string]any{
					"id":   "a",
					"name": "alpha",
				},
			},
		})
		if err != nil {
			t.Fatalf("Request returned error: %v", err)
		}
	})

	t.Run("non_json_response_falls_back_to_text", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "pong")
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		value, err := client.Request(context.Background(), managedserverdomain.RequestSpec{
			Method: http.MethodGet,
			Path:   "/health",
		})
		if err != nil {
			t.Fatalf("Request returned error: %v", err)
		}
		if value.Value != "pong" {
			t.Fatalf("expected text fallback response, got %#v", value.Value)
		}
	})

	t.Run("validates_method_and_path", func(t *testing.T) {
		t.Parallel()

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.Request(context.Background(), managedserverdomain.RequestSpec{
			Method: "",
			Path:   "/test",
		})
		assertTypedCategory(t, err, faults.ValidationError)

		_, err = client.Request(context.Background(), managedserverdomain.RequestSpec{
			Method: http.MethodGet,
			Path:   "",
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestCachedListJQCodeCachesCompiledExpressions(t *testing.T) {
	t.Parallel()

	expression := `.[] | .id`

	codeOne, err := cachedListJQCode(expression)
	if err != nil {
		t.Fatalf("cachedListJQCode first call returned error: %v", err)
	}
	codeTwo, err := cachedListJQCode(expression)
	if err != nil {
		t.Fatalf("cachedListJQCode second call returned error: %v", err)
	}

	if codeOne == nil || codeTwo == nil {
		t.Fatal("expected compiled jq code")
	}
	if codeOne != codeTwo {
		t.Fatal("expected compiled jq code to be cached and reused")
	}
}

func TestStatusMappingAndExists(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		status   int
		category faults.ErrorCategory
	}{
		{name: "unauthorized_maps_auth", status: http.StatusUnauthorized, category: faults.AuthError},
		{name: "not_found_maps_not_found", status: http.StatusNotFound, category: faults.NotFoundError},
		{name: "conflict_maps_conflict", status: http.StatusConflict, category: faults.ConflictError},
		{name: "bad_request_maps_validation", status: http.StatusBadRequest, category: faults.ValidationError},
		{name: "internal_maps_transport", status: http.StatusInternalServerError, category: faults.TransportError},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = fmt.Fprint(w, "test-body")
			}))
			t.Cleanup(server.Close)

			client := mustManagedServerClient(t, config.HTTPServer{
				BaseURL: server.URL,
				Auth: &config.HTTPAuth{
					CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
				},
			})

			md := metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			}
			_, err := client.Get(context.Background(), resource.Resource{
				LogicalPath: "/customers/acme",
			}, md)
			assertTypedCategory(t, err, test.category)
			if !strings.Contains(err.Error(), "test-body") {
				t.Fatalf("expected response body context in error, got %q", err.Error())
			}
		})
	}

	t.Run("exists_returns_false_on_404", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		md := metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationGet): {Path: "/resource"},
			},
		}
		exists, err := client.Exists(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
		}, md)
		if err != nil {
			t.Fatalf("Exists returned error: %v", err)
		}
		if exists {
			t.Fatal("expected false when resource returns 404")
		}
	})
}

func TestListResponseShapesAndAliasRules(t *testing.T) {
	t.Parallel()

	t.Run("array_shape_with_id_fallback_and_sorting", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"b"},{"id":"a"}]`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		items, err := client.List(context.Background(), "/customers", metadata.ResourceMetadata{
			IDAttribute: "/id",
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].LogicalPath != "/customers/a" || items[1].LogicalPath != "/customers/b" {
			t.Fatalf("expected deterministic sorted output, got %#v", items)
		}
	})

	t.Run("list_operation_applies_jq_and_collection_template", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/admin/realms/publico-br/components" {
				t.Fatalf("expected list request path /admin/realms/publico-br/components, got %q", r.URL.Path)
			}
			_, _ = fmt.Fprint(
				w,
				`[
				  {"id":"ldap-id","name":"user-registry","providerId":"ldap"},
				  {"id":"oidc-id","name":"identity-provider","providerId":"oidc"}
				]`,
			)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		items, err := client.List(context.Background(), "/admin/realms/publico-br", metadata.ResourceMetadata{
			IDAttribute:          "/id",
			AliasAttribute:       "/name",
			RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Transforms: jqMutation(`[ .[] | select(.providerId == "ldap") ]`),
				},
			},
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected jq-filtered list with 1 item, got %d", len(items))
		}
		if items[0].LogicalPath != "/admin/realms/publico-br/user-registry" {
			t.Fatalf("unexpected filtered logical path %#v", items[0].LogicalPath)
		}
	})

	t.Run("list_operation_jq_resource_function_requires_context_resolver", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(
				w,
				`[
				  {"id":"widget-a","name":"alpha","parentId":"project-primary"},
				  {"id":"widget-b","name":"beta","parentId":"project-secondary"}
				]`,
			)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/api/projects/widgets", metadata.ResourceMetadata{
			IDAttribute:    "/id",
			AliasAttribute: "/name",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Path:       `/api/projects/widgets`,
					Transforms: jqMutation(`[ .[] | select(.parentId == (resource("/api/projects/current") | .id)) ]`),
				},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("list_operation_jq_resource_function_uses_context_resolver", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/admin/realms/publico-br/components" {
				t.Fatalf("expected list request path /admin/realms/publico-br/components, got %q", r.URL.Path)
			}
			_, _ = fmt.Fprint(
				w,
				`[
				  {"id":"mapper-a","name":"alpha","parentId":"ldap-id"},
				  {"id":"mapper-b","name":"beta","parentId":"other-id"}
				]`,
			)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		var resolverCalls int32
		ctx := managedserverdomain.WithListJQResourceResolver(
			context.Background(),
			func(_ context.Context, logicalPath string) (resource.Value, error) {
				atomic.AddInt32(&resolverCalls, 1)
				if logicalPath != "/admin/realms/publico-br/user-registry/ldap-test" {
					t.Fatalf("unexpected resolved logical path %q", logicalPath)
				}
				return map[string]any{"id": "ldap-id"}, nil
			},
		)

		items, err := client.List(ctx, "/admin/realms/publico-br/user-registry/ldap-test/mappers", metadata.ResourceMetadata{
			IDAttribute:          "/id",
			AliasAttribute:       "/name",
			RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Transforms: jqMutation(`[ .[] | select(.parentId == (resource("/admin/realms/{{.realm}}/user-registry/{{.provider}}/") | .id)) ]`),
				},
			},
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}

		if len(items) != 1 {
			t.Fatalf("expected jq-filtered list with 1 item, got %d", len(items))
		}
		if items[0].LogicalPath != "/admin/realms/publico-br/user-registry/ldap-test/mappers/alpha" {
			t.Fatalf("unexpected filtered logical path %#v", items[0].LogicalPath)
		}
		if got := atomic.LoadInt32(&resolverCalls); got != 1 {
			t.Fatalf("expected context resolver to be called once, got %d", got)
		}
	})

	t.Run("list_operation_jq_resource_function_renders_name_from_parent_path", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/admin/realms/publico-br/components" {
				t.Fatalf("expected list request path /admin/realms/publico-br/components, got %q", r.URL.Path)
			}
			_, _ = fmt.Fprint(
				w,
				`[
				  {"id":"mapper-a","name":"alpha","parentId":"ldap-id"},
				  {"id":"mapper-b","name":"beta","parentId":"other-id"}
				]`,
			)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		var resolverCalls int32
		ctx := managedserverdomain.WithListJQResourceResolver(
			context.Background(),
			func(_ context.Context, logicalPath string) (resource.Value, error) {
				atomic.AddInt32(&resolverCalls, 1)
				if logicalPath != "/admin/realms/publico-br/user-registry/AD" {
					t.Fatalf("unexpected resolved logical path %q", logicalPath)
				}
				return map[string]any{"id": "ldap-id"}, nil
			},
		)

		items, err := client.List(ctx, "/admin/realms/publico-br/user-registry/AD/mappers", metadata.ResourceMetadata{
			IDAttribute:          "/id",
			AliasAttribute:       "/name",
			RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Transforms: jqMutation(`[ .[] | select(.parentId == (resource("/admin/realms/{{.realm}}/user-registry/{{.name}}/") | .id)) ]`),
				},
			},
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}

		if len(items) != 1 {
			t.Fatalf("expected jq-filtered list with 1 item, got %d", len(items))
		}
		if items[0].LogicalPath != "/admin/realms/publico-br/user-registry/AD/mappers/alpha" {
			t.Fatalf("unexpected filtered logical path %#v", items[0].LogicalPath)
		}
		if got := atomic.LoadInt32(&resolverCalls); got != 1 {
			t.Fatalf("expected context resolver to be called once, got %d", got)
		}
	})

	t.Run("invalid_list_jq_returns_validation_error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"ldap-id","name":"user-registry"}]`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/admin/realms/publico-br", metadata.ResourceMetadata{
			IDAttribute:    "/id",
			AliasAttribute: "/name",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Transforms: jqMutation("[ .[] | select(.providerId == ]"),
				},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("invalid_list_jq_resource_argument_returns_validation_error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"mapper-a","name":"alpha","parentId":"ldap-id"}]`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/admin/realms/publico-br/user-registry/ldap-test/mappers", metadata.ResourceMetadata{
			IDAttribute:    "/id",
			AliasAttribute: "/name",
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationList): {
					Path:       `/admin/realms/publico-br/user-registry/ldap-test/mappers`,
					Transforms: jqMutation(`[ .[] | select(.parentId == (resource(1) | .id)) ]`),
				},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("object_items_shape_supported", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"items":[{"alias":"one","id":"1"},{"id":"2"}]}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		items, err := client.List(context.Background(), "/customers", metadata.ResourceMetadata{
			AliasAttribute: "/alias",
			IDAttribute:    "/id",
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].LogicalPath != "/customers/2" || items[1].LogicalPath != "/customers/one" {
			t.Fatalf("unexpected logical paths: %#v", items)
		}
	})

	t.Run("object_single_array_field_shape_supported", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"realms":[{"realm":"b"},{"realm":"a"}]}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		items, err := client.List(context.Background(), "/admin/realms", metadata.ResourceMetadata{
			IDAttribute: "/realm",
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		if items[0].LogicalPath != "/admin/realms/a" || items[1].LogicalPath != "/admin/realms/b" {
			t.Fatalf("expected deterministic sorted output, got %#v", items)
		}
	})

	t.Run("object_multiple_array_fields_returns_validation_error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"groups":[{"id":"g"}],"realms":[{"id":"r"}]}`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/customers", metadata.ResourceMetadata{
			IDAttribute: "/id",
		})
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguous list response validation error, got %v", err)
		}
	})

	t.Run("duplicate_alias_returns_conflict", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"same"},{"id":"same"}]`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/customers", metadata.ResourceMetadata{
			IDAttribute: "/id",
		})
		assertTypedCategory(t, err, faults.ConflictError)
	})

	t.Run("missing_alias_and_id_returns_validation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"name":"no-id"}]`)
		}))
		t.Cleanup(server.Close)

		client := mustManagedServerClient(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeaders: []config.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "token"}},
			},
		})

		_, err := client.List(context.Background(), "/customers", metadata.ResourceMetadata{
			AliasAttribute: "/alias",
			IDAttribute:    "/id",
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func writeTLSClientPairFiles(t *testing.T) (string, string, string) {
	t.Helper()

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tlsServer.Close()

	if len(tlsServer.TLS.Certificates) == 0 {
		t.Fatal("expected test TLS server certificate")
	}
	certificate := tlsServer.TLS.Certificates[0]
	if len(certificate.Certificate) == 0 {
		t.Fatal("expected test TLS certificate chain")
	}

	certBuffer := bytes.NewBuffer(nil)
	for _, certDER := range certificate.Certificate {
		if err := pem.Encode(certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
			t.Fatalf("failed to encode certificate pem: %v", err)
		}
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	if len(privateKeyPEM) == 0 {
		t.Fatal("expected private key pem bytes")
	}

	tempDir := t.TempDir()
	caCertFile := filepath.Join(tempDir, "ca-cert.pem")
	clientCertFile := filepath.Join(tempDir, "client-cert.pem")
	clientKeyFile := filepath.Join(tempDir, "client-key.pem")

	if err := os.WriteFile(caCertFile, certBuffer.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write ca cert file: %v", err)
	}
	if err := os.WriteFile(clientCertFile, certBuffer.Bytes(), 0o600); err != nil {
		t.Fatalf("failed to write client cert file: %v", err)
	}
	if err := os.WriteFile(clientKeyFile, privateKeyPEM, 0o600); err != nil {
		t.Fatalf("failed to write client key file: %v", err)
	}

	return caCertFile, clientCertFile, clientKeyFile
}

func mustManagedServerClient(
	t *testing.T,
	cfg config.HTTPServer,
	opts ...ClientOption,
) *Client {
	t.Helper()

	client, err := NewClient(cfg, opts...)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func TestJQCacheBounding(t *testing.T) {
	t.Parallel()

	// Reset cache state for this test.
	jqCacheMu.Lock()
	jqCacheMap = make(map[string]*gojq.Code)
	jqCacheOrder = nil
	jqCacheMu.Unlock()

	for i := 0; i < maxJQCacheEntries+50; i++ {
		expr := fmt.Sprintf(".field%d", i)
		_, err := cachedListJQCode(expr)
		if err != nil {
			t.Fatalf("cachedListJQCode(%q) returned error: %v", expr, err)
		}
	}

	jqCacheMu.Lock()
	size := len(jqCacheMap)
	orderLen := len(jqCacheOrder)
	jqCacheMu.Unlock()

	if size > maxJQCacheEntries {
		t.Fatalf("JQ cache exceeded max entries: got %d, max %d", size, maxJQCacheEntries)
	}
	if orderLen > maxJQCacheEntries {
		t.Fatalf("JQ cache order exceeded max entries: got %d, max %d", orderLen, maxJQCacheEntries)
	}
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
