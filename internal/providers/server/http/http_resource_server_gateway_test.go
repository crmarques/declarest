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
	"strings"
	"sync/atomic"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestNewHTTPResourceServerGatewayValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing_base_url", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPResourceServerGateway(config.HTTPServer{
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("oauth2_grant_type_not_supported", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPResourceServerGateway(config.HTTPServer{
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

		_, err := NewHTTPResourceServerGateway(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
			TLS: &config.TLS{
				ClientCertFile: "/tmp/only-cert.pem",
			},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("openapi_http_url_not_allowed", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPResourceServerGateway(config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
			OpenAPI: "http://example.com/openapi.json",
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestBuildRequestFromMetadataDefaultsAndHeaders(t *testing.T) {
	t.Parallel()

	gateway := mustGateway(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			BearerToken: &config.BearerTokenAuth{Token: "token"},
		},
		DefaultHeaders: map[string]string{
			"X-Default":  "base",
			"X-Override": "base",
		},
	})

	spec, err := gateway.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath: "/customers/acme",
		Payload:     map[string]any{"name": "ACME"},
		Metadata: metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path:    "customers",
					Query:   map[string]string{"expand": "true"},
					Headers: map[string]string{"X-Override": "operation"},
				},
			},
		},
	}, metadata.OperationCreate)
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

func TestBuildRequestFromMetadataRendersTemplates(t *testing.T) {
	t.Parallel()

	gateway := mustGateway(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			BearerToken: &config.BearerTokenAuth{Token: "token"},
		},
	})

	spec, err := gateway.BuildRequestFromMetadata(context.Background(), resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli",
		CollectionPath: "/admin/realms/platform/clients",
		Payload: map[string]any{
			"realm":    "platform",
			"clientId": "declarest-cli",
		},
		Metadata: metadata.ResourceMetadata{
			Operations: map[string]metadata.OperationSpec{
				string(metadata.OperationCreate): {
					Path: "/admin/realms/{{.realm}}/clients",
					Query: map[string]string{
						"clientId": "{{.clientId}}",
					},
				},
			},
		},
	}, metadata.OperationCreate)
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

	gateway := mustGateway(t, config.HTTPServer{
		BaseURL: "https://example.com/api",
		Auth: &config.HTTPAuth{
			BearerToken: &config.BearerTokenAuth{Token: "token"},
		},
		OpenAPI: specPath,
	})

	t.Run("fills_missing_fields_from_openapi", func(t *testing.T) {
		t.Parallel()

		spec, err := gateway.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationCreate): {
						Path: "/customers",
					},
				},
			},
		}, metadata.OperationCreate)
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

		_, err := gateway.BuildRequestFromMetadata(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationDelete): {
						Path: "/customers",
					},
				},
			},
		}, metadata.OperationDelete)
		assertTypedCategory(t, err, faults.ValidationError)
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

	gateway := mustGateway(t, config.HTTPServer{
		BaseURL: server.URL,
		Auth: &config.HTTPAuth{
			BearerToken: &config.BearerTokenAuth{Token: "token"},
		},
		TLS: &config.TLS{
			InsecureSkipVerify: true,
		},
		OpenAPI: server.URL + "/openapi.json",
	})

	valueOne, err := gateway.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("GetOpenAPISpec first call returned error: %v", err)
	}
	valueTwo, err := gateway.GetOpenAPISpec(context.Background())
	if err != nil {
		t.Fatalf("GetOpenAPISpec second call returned error: %v", err)
	}

	if valueOne == nil || valueTwo == nil {
		t.Fatal("expected non-nil OpenAPI payload")
	}
	if got := atomic.LoadInt32(&openAPICalls); got != 1 {
		t.Fatalf("expected one openapi fetch, got %d", got)
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BasicAuth: &config.BasicAuth{Username: "user", Password: "pass"},
			},
		})

		_, err := gateway.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		})
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token-123"},
			},
		})

		_, err := gateway.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		})
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				CustomHeader: &config.HeaderTokenAuth{Header: "X-Auth-Token", Token: "custom-token"},
			},
		})

		_, err := gateway.Get(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		})
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

		gateway := mustGateway(t, config.HTTPServer{
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

		resourceInfo := resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		}
		if _, err := gateway.Get(context.Background(), resourceInfo); err != nil {
			t.Fatalf("first Get returned error: %v", err)
		}
		if _, err := gateway.Get(context.Background(), resourceInfo); err != nil {
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

		gateway := mustGateway(t, config.HTTPServer{
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

		resourceInfo := resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		}

		var debugOutput bytes.Buffer
		ctx := debugctx.WithEnabled(context.Background(), true)
		ctx = debugctx.WithWriter(ctx, &debugOutput)

		if _, err := gateway.Get(ctx, resourceInfo); err != nil {
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
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

		_, err := gateway.Get(ctx, resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		})
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
}

func TestAdHocRequests(t *testing.T) {
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		value, err := gateway.AdHoc(context.Background(), http.MethodGet, "/test", nil)
		if err != nil {
			t.Fatalf("AdHoc returned error: %v", err)
		}

		output, ok := value.(map[string]any)
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		_, err := gateway.AdHoc(context.Background(), http.MethodPost, "/test", map[string]any{
			"id":   "a",
			"name": "alpha",
		})
		if err != nil {
			t.Fatalf("AdHoc returned error: %v", err)
		}
	})

	t.Run("non_json_response_falls_back_to_text", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "pong")
		}))
		t.Cleanup(server.Close)

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		value, err := gateway.AdHoc(context.Background(), http.MethodGet, "/health", nil)
		if err != nil {
			t.Fatalf("AdHoc returned error: %v", err)
		}
		if value != "pong" {
			t.Fatalf("expected text fallback response, got %#v", value)
		}
	})

	t.Run("validates_method_and_path", func(t *testing.T) {
		t.Parallel()

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: "https://example.com",
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		_, err := gateway.AdHoc(context.Background(), "", "/test", nil)
		assertTypedCategory(t, err, faults.ValidationError)

		_, err = gateway.AdHoc(context.Background(), http.MethodGet, "", nil)
		assertTypedCategory(t, err, faults.ValidationError)
	})
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

			gateway := mustGateway(t, config.HTTPServer{
				BaseURL: server.URL,
				Auth: &config.HTTPAuth{
					BearerToken: &config.BearerTokenAuth{Token: "token"},
				},
			})

			_, err := gateway.Get(context.Background(), resource.Resource{
				LogicalPath: "/customers/acme",
				Metadata: metadata.ResourceMetadata{
					Operations: map[string]metadata.OperationSpec{
						string(metadata.OperationGet): {Path: "/resource"},
					},
				},
			})
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		exists, err := gateway.Exists(context.Background(), resource.Resource{
			LogicalPath: "/customers/acme",
			Metadata: metadata.ResourceMetadata{
				Operations: map[string]metadata.OperationSpec{
					string(metadata.OperationGet): {Path: "/resource"},
				},
			},
		})
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

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		items, err := gateway.List(context.Background(), "/customers", metadata.ResourceMetadata{
			IDFromAttribute: "id",
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

	t.Run("object_items_shape_supported", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `{"items":[{"alias":"one","id":"1"},{"id":"2"}]}`)
		}))
		t.Cleanup(server.Close)

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		items, err := gateway.List(context.Background(), "/customers", metadata.ResourceMetadata{
			AliasFromAttribute: "alias",
			IDFromAttribute:    "id",
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

	t.Run("duplicate_alias_returns_conflict", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"id":"same"},{"id":"same"}]`)
		}))
		t.Cleanup(server.Close)

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		_, err := gateway.List(context.Background(), "/customers", metadata.ResourceMetadata{
			IDFromAttribute: "id",
		})
		assertTypedCategory(t, err, faults.ConflictError)
	})

	t.Run("missing_alias_and_id_returns_validation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, `[{"name":"no-id"}]`)
		}))
		t.Cleanup(server.Close)

		gateway := mustGateway(t, config.HTTPServer{
			BaseURL: server.URL,
			Auth: &config.HTTPAuth{
				BearerToken: &config.BearerTokenAuth{Token: "token"},
			},
		})

		_, err := gateway.List(context.Background(), "/customers", metadata.ResourceMetadata{
			AliasFromAttribute: "alias",
			IDFromAttribute:    "id",
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

func mustGateway(t *testing.T, cfg config.HTTPServer) *HTTPResourceServerGateway {
	t.Helper()

	gateway, err := NewHTTPResourceServerGateway(cfg)
	if err != nil {
		t.Fatalf("NewHTTPResourceServerGateway returned error: %v", err)
	}
	return gateway
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
