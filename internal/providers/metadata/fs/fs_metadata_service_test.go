package fsmetadata

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func TestFSMetadataGetSetUnset(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	resourceMetadata := metadatadomain.ResourceMetadata{
		SecretsFromAttributes: []string{"credentials.authValue"},
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {Path: "/api/customers/{{.id}}"},
		},
	}
	if err := service.Set(ctx, "/customers/acme", resourceMetadata); err != nil {
		t.Fatalf("Set resource metadata returned error: %v", err)
	}

	gotResource, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get resource metadata returned error: %v", err)
	}
	if !reflect.DeepEqual(resourceMetadata, gotResource) {
		t.Fatalf("unexpected resource metadata: %+v", gotResource)
	}

	collectionMetadata := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationList): {Path: "/api/customers"},
		},
	}
	if err := service.Set(ctx, "/customers/_", collectionMetadata); err != nil {
		t.Fatalf("Set collection metadata returned error: %v", err)
	}

	gotCollection, err := service.Get(ctx, "/customers/_")
	if err != nil {
		t.Fatalf("Get collection metadata returned error: %v", err)
	}
	if !reflect.DeepEqual(collectionMetadata, gotCollection) {
		t.Fatalf("unexpected collection metadata: %+v", gotCollection)
	}

	wildcardMetadata := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {Headers: map[string]string{"X-Scope": "wildcard"}},
		},
	}
	if err := service.Set(ctx, "/customers/*", wildcardMetadata); err != nil {
		t.Fatalf("Set wildcard metadata returned error: %v", err)
	}

	if err := service.Unset(ctx, "/customers/*"); err != nil {
		t.Fatalf("Unset wildcard metadata returned error: %v", err)
	}

	_, err = service.Get(ctx, "/customers/*")
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestFSMetadataCollectionPathWithTrailingSlash(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	metadata := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationList): {Path: "/api/realms"},
		},
	}

	if err := service.Set(ctx, "/admin/realms/", metadata); err != nil {
		t.Fatalf("Set trailing-slash collection metadata returned error: %v", err)
	}

	got, err := service.Get(ctx, "/admin/realms/")
	if err != nil {
		t.Fatalf("Get trailing-slash collection metadata returned error: %v", err)
	}
	if !reflect.DeepEqual(metadata, got) {
		t.Fatalf("unexpected metadata for trailing-slash collection path: %+v", got)
	}

	if err := service.Unset(ctx, "/admin/realms/"); err != nil {
		t.Fatalf("Unset trailing-slash collection metadata returned error: %v", err)
	}

	_, err = service.Get(ctx, "/admin/realms/")
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestFSMetadataResolveForPathWildcardRules(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/_", metadatadomain.ResourceMetadata{
		Suppress: []string{"/root"},
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Headers: map[string]string{
					"X-Order": "collection",
					"X-Root":  "true",
				},
			},
		},
	})

	mustSetMetadata(t, service, ctx, "/customers/*", metadatadomain.ResourceMetadata{
		Suppress: []string{"/wild-1"},
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Headers: map[string]string{
					"X-Wild-Order": "first",
					"X-Order":      "wild-1",
				},
			},
		},
	})

	mustSetMetadata(t, service, ctx, "/customers/a*", metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Headers: map[string]string{
					"X-Wild-Order": "second",
					"X-Order":      "wild-2",
				},
			},
		},
	})

	mustSetMetadata(t, service, ctx, "/customers/acme/_", metadatadomain.ResourceMetadata{
		Suppress: []string{},
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Headers: map[string]string{
					"X-Order":   "literal",
					"X-Literal": "true",
				},
			},
		},
	})

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "/api/customers/{{.id}}",
				Headers: map[string]string{
					"X-Resource": "true",
				},
			},
		},
	})

	resolved, err := service.ResolveForPath(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("ResolveForPath returned error: %v", err)
	}

	if len(resolved.Suppress) != 0 {
		t.Fatalf("expected literal array replacement to clear suppress list, got %+v", resolved.Suppress)
	}

	headers := resolved.Operations[string(metadatadomain.OperationGet)].Headers
	if headers["X-Wild-Order"] != "second" {
		t.Fatalf("expected lexicographic wildcard merge order, got %+v", headers)
	}
	if headers["X-Order"] != "literal" {
		t.Fatalf("expected literal to override wildcard at same depth, got %+v", headers)
	}
	if headers["X-Root"] != "true" || headers["X-Literal"] != "true" || headers["X-Resource"] != "true" {
		t.Fatalf("expected merged headers from all layers, got %+v", headers)
	}
}

func TestFSMetadataResolveForPathSecretsFromAttributesLayering(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/_", metadatadomain.ResourceMetadata{
		SecretsFromAttributes: []string{"credentials.rootSecret"},
	})
	mustSetMetadata(t, service, ctx, "/customers/*", metadatadomain.ResourceMetadata{
		SecretsFromAttributes: []string{"credentials.wildcardSecret"},
	})
	mustSetMetadata(t, service, ctx, "/customers/acme/_", metadatadomain.ResourceMetadata{
		SecretsFromAttributes: []string{"credentials.literalSecret"},
	})

	resolved, err := service.ResolveForPath(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("ResolveForPath returned error: %v", err)
	}
	expected := []string{"credentials.literalSecret"}
	if !reflect.DeepEqual(expected, resolved.SecretsFromAttributes) {
		t.Fatalf("expected literal layer to replace secret attributes, got %#v", resolved.SecretsFromAttributes)
	}
}

func TestFSMetadataResolveForPathIntermediaryPlaceholderSelectors(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir, "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "realm",
		AliasFromAttribute: "realm",
	})
	mustSetMetadata(t, service, ctx, "/admin/realms/_/clients", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "clientId",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationCreate): {
				Path: "/admin/realms/{{.realm}}/clients",
			},
		},
	})

	resolved, err := service.ResolveForPath(ctx, "/admin/realms/master/clients/broker")
	if err != nil {
		t.Fatalf("ResolveForPath returned error: %v", err)
	}
	if resolved.IDFromAttribute != "id" {
		t.Fatalf("expected clients idFromAttribute from intermediary placeholder metadata, got %q", resolved.IDFromAttribute)
	}
	if resolved.AliasFromAttribute != "clientId" {
		t.Fatalf(
			"expected clients aliasFromAttribute from intermediary placeholder metadata, got %q",
			resolved.AliasFromAttribute,
		)
	}
	createPath := resolved.Operations[string(metadatadomain.OperationCreate)].Path
	if createPath != "/admin/realms/{{.realm}}/clients" {
		t.Fatalf("expected create path from clients metadata, got %q", createPath)
	}
}

func TestFSMetadataResolveCollectionChildrenSupportsIntermediarySelectors(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry/_", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "name",
		CollectionPath:     "/admin/realms/{{.realm}}/components",
	})
	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry/_/mappers/_", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "name",
		CollectionPath:     "/admin/realms/{{.realm}}/components",
	})

	children, err := service.ResolveCollectionChildren(
		ctx,
		"/admin/realms/master/user-registry/AD PRD",
	)
	if err != nil {
		t.Fatalf("ResolveCollectionChildren returned error: %v", err)
	}
	if !reflect.DeepEqual(children, []string{"mappers"}) {
		t.Fatalf("expected metadata child branch [mappers], got %#v", children)
	}
}

func TestFSMetadataHasCollectionWildcardChild(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/authentication/flows/_/executions/_", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "displayName",
	})

	ok, err := service.HasCollectionWildcardChild(
		ctx,
		"/admin/realms/master/authentication/flows/test/executions",
	)
	if err != nil {
		t.Fatalf("HasCollectionWildcardChild returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected wildcard child info to enable fallback")
	}

	ok, err = service.HasCollectionWildcardChild(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("HasCollectionWildcardChild returned error: %v", err)
	}
	if ok {
		t.Fatal("expected no wildcard child under path without metadata selectors")
	}
}

func TestFSMetadataRenderOperationSpec(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "slug",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "/api{{.collectionPath}}/{{.alias}}/{{.remoteID}}",
				Headers: map[string]string{
					"X-Tenant": "{{.tenant}}",
				},
			},
		},
	})

	spec, err := service.RenderOperationSpec(ctx, "/customers/acme", metadatadomain.OperationGet, map[string]any{
		"id":     "42",
		"slug":   "acme",
		"tenant": "north",
	})
	if err != nil {
		t.Fatalf("RenderOperationSpec returned error: %v", err)
	}

	if spec.Path != "/api/customers/acme/42" {
		t.Fatalf("unexpected rendered path %q", spec.Path)
	}
	if spec.Headers["X-Tenant"] != "north" {
		t.Fatalf("unexpected rendered headers %+v", spec.Headers)
	}
}

func TestFSMetadataRenderOperationSpecSupportsCollectionPathIndirection(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry", metadatadomain.ResourceMetadata{
		IDFromAttribute: "id",
		CollectionPath:  "/admin/realms/{{.realm}}/components",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "./{{.id}}",
			},
		},
	})

	getSpec, err := service.RenderOperationSpec(
		ctx,
		"/admin/realms/platform/user-registry",
		metadatadomain.OperationGet,
		map[string]any{
			"id": "123456",
		},
	)
	if err != nil {
		t.Fatalf("RenderOperationSpec(get) returned error: %v", err)
	}
	if getSpec.Path != "/admin/realms/platform/components/123456" {
		t.Fatalf("unexpected rendered get path %q", getSpec.Path)
	}

	createSpec, err := service.RenderOperationSpec(
		ctx,
		"/admin/realms/platform/user-registry",
		metadatadomain.OperationCreate,
		map[string]any{
			"id": "123456",
		},
	)
	if err != nil {
		t.Fatalf("RenderOperationSpec(create) returned error: %v", err)
	}
	if createSpec.Path != "/admin/realms/platform/components" {
		t.Fatalf("unexpected rendered create path %q", createSpec.Path)
	}
}

func TestFSMetadataRenderOperationSpecSupportsResourceFormatTemplateFunc(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "yaml")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "id",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path:   "/api/customers/{{.id}}",
				Accept: "application/{{resource_format .}}",
			},
		},
	})

	spec, err := service.RenderOperationSpec(ctx, "/customers/acme", metadatadomain.OperationGet, map[string]any{
		"id": "acme",
	})
	if err != nil {
		t.Fatalf("RenderOperationSpec returned error: %v", err)
	}
	if spec.Accept != "application/yaml" {
		t.Fatalf("expected accept application/yaml, got %q", spec.Accept)
	}
}

func TestFSMetadataValidation(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	err := service.Set(ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			"fetch": {Path: "/api/customers"},
		},
	})
	assertTypedCategory(t, err, faults.ValidationError)

	_, err = service.ResolveForPath(ctx, "/customers/*")
	assertTypedCategory(t, err, faults.ValidationError)
}

func TestFSMetadataSetOmitsNilFieldsFromStoredJSON(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir, "")
	ctx := context.Background()

	metadata := metadatadomain.ResourceMetadata{
		IDFromAttribute:       "id",
		AliasFromAttribute:    "clientId",
		SecretsFromAttributes: []string{"secret"},
	}

	if err := service.Set(ctx, "/admin/realms/_/clients/_", metadata); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	filePath := filepath.Join(baseDir, "admin", "realms", "_", "clients", "_", "metadata.json")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read metadata file %q: %v", filePath, err)
	}
	if !strings.HasSuffix(string(content), "\n") {
		t.Fatalf("expected metadata file to end with newline, got %q", string(content))
	}

	if strings.Contains(string(content), "null") {
		t.Fatalf("expected metadata file without null values, got %s", string(content))
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("failed to decode metadata file: %v", err)
	}

	resourceInfo, hasResourceInfo := decoded["resourceInfo"].(map[string]any)
	if !hasResourceInfo {
		t.Fatalf("expected resourceInfo object, got %#v", decoded["resourceInfo"])
	}
	if _, found := resourceInfo["secretInAttributes"]; !found {
		t.Fatalf("expected secretInAttributes under resourceInfo, got %#v", resourceInfo)
	}
	if _, found := decoded["operationInfo"]; found {
		t.Fatalf("expected operationInfo key to be omitted when nil, got %v", decoded["operationInfo"])
	}
}

func TestFSMetadataSetPreservesExplicitEmptyCollections(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir, "")
	ctx := context.Background()

	metadata := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path:     "/api/customers/{{.id}}",
				Query:    map[string]string{},
				Headers:  map[string]string{},
				Filter:   []string{},
				Suppress: []string{},
			},
		},
		Filter:   []string{},
		Suppress: []string{},
	}

	if err := service.Set(ctx, "/customers/acme", metadata); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	filePath := filepath.Join(baseDir, "customers", "acme", "metadata.json")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read metadata file %q: %v", filePath, err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("failed to decode metadata file: %v", err)
	}

	operationInfo, hasOperationInfo := decoded["operationInfo"].(map[string]any)
	if !hasOperationInfo {
		t.Fatalf("expected operationInfo object, got %#v", decoded["operationInfo"])
	}

	defaults, hasDefaults := operationInfo["defaults"].(map[string]any)
	if !hasDefaults {
		t.Fatalf("expected operationInfo defaults object, got %#v", operationInfo["defaults"])
	}
	defaultPayload, hasDefaultPayload := defaults["payload"].(map[string]any)
	if !hasDefaultPayload {
		t.Fatalf("expected operationInfo.defaults.payload object, got %#v", defaults["payload"])
	}

	filter, hasFilter := defaultPayload["filterAttributes"].([]any)
	if !hasFilter || len(filter) != 0 {
		t.Fatalf("expected explicit empty filter array, got %#v", defaultPayload["filterAttributes"])
	}
	suppress, hasSuppress := defaultPayload["suppressAttributes"].([]any)
	if !hasSuppress || len(suppress) != 0 {
		t.Fatalf("expected explicit empty suppress array, got %#v", defaultPayload["suppressAttributes"])
	}

	getSpec, hasGet := operationInfo["getResource"].(map[string]any)
	if !hasGet {
		t.Fatalf("expected get operation metadata, got %#v", operationInfo["getResource"])
	}

	query, hasQuery := getSpec["query"].(map[string]any)
	if !hasQuery || len(query) != 0 {
		t.Fatalf("expected explicit empty query object, got %#v", getSpec["query"])
	}
	headers, hasHeaders := getSpec["httpHeaders"].([]any)
	if !hasHeaders || len(headers) != 0 {
		t.Fatalf("expected explicit empty httpHeaders array, got %#v", getSpec["httpHeaders"])
	}
	payload, hasPayload := getSpec["payload"].(map[string]any)
	if !hasPayload {
		t.Fatalf("expected get operation payload object, got %#v", getSpec["payload"])
	}
	specFilter, hasSpecFilter := payload["filterAttributes"].([]any)
	if !hasSpecFilter || len(specFilter) != 0 {
		t.Fatalf("expected explicit empty operation filter array, got %#v", payload["filterAttributes"])
	}
	specSuppress, hasSpecSuppress := payload["suppressAttributes"].([]any)
	if !hasSpecSuppress || len(specSuppress) != 0 {
		t.Fatalf("expected explicit empty operation suppress array, got %#v", payload["suppressAttributes"])
	}
}

func TestFSMetadataGetSupportsOperationURLPathSyntax(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir, "")
	ctx := context.Background()

	filePath := filepath.Join(baseDir, "admin", "realms", "_", "user-registry", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create metadata directory %q: %v", filepath.Dir(filePath), err)
	}

	payload := `{
  "resourceInfo": {
    "collectionPath": "/admin/realms/{{.realm}}/components"
  },
  "operationInfo": {
    "getResource": {
      "url": {
        "path": "./{{.id}}"
      }
    }
  }
}
`
	if err := os.WriteFile(filePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("failed to write metadata file %q: %v", filePath, err)
	}

	decoded, err := service.Get(ctx, "/admin/realms/_/user-registry")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if decoded.CollectionPath != "/admin/realms/{{.realm}}/components" {
		t.Fatalf("unexpected collectionPath: %q", decoded.CollectionPath)
	}
	if decoded.Operations[string(metadatadomain.OperationGet)].Path != "./{{.id}}" {
		t.Fatalf("unexpected get path: %#v", decoded.Operations[string(metadatadomain.OperationGet)])
	}
}

func mustSetMetadata(
	t *testing.T,
	service *FSMetadataService,
	ctx context.Context,
	logicalPath string,
	metadata metadatadomain.ResourceMetadata,
) {
	t.Helper()

	if err := service.Set(ctx, logicalPath, metadata); err != nil {
		t.Fatalf("failed to set metadata %q: %v", logicalPath, err)
	}
}

func writeRawMetadataFile(t *testing.T, filePath string, metadata metadatadomain.ResourceMetadata) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create metadata directory %q: %v", filepath.Dir(filePath), err)
	}

	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("failed to encode metadata for %q: %v", filePath, err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(filePath, encoded, 0o644); err != nil {
		t.Fatalf("failed to write metadata file %q: %v", filePath, err)
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
