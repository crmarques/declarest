package fsmetadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"go.yaml.in/yaml/v3"
)

func TestFSMetadataGetSetUnset(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	resourceMetadata := metadatadomain.ResourceMetadata{
		SecretAttributes: []string{"/credentials/authValue"},
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

	service := NewFSMetadataService(t.TempDir())
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

func TestFSMetadataGetSupportsJSONAndPrefersYAML(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	writeMetadataFixture(t, filepath.Join(baseDir, "customers", "acme", "metadata.json"), false, metadatadomain.ResourceMetadata{
		IDAttribute:    "/json-id",
		AliasAttribute: "/json-alias",
	})

	gotJSON, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get json metadata returned error: %v", err)
	}
	if gotJSON.IDAttribute != "/json-id" || gotJSON.AliasAttribute != "/json-alias" {
		t.Fatalf("expected json metadata fallback, got %+v", gotJSON)
	}

	writeMetadataFixture(t, filepath.Join(baseDir, "customers", "acme", "metadata.yaml"), true, metadatadomain.ResourceMetadata{
		IDAttribute:    "/yaml-id",
		AliasAttribute: "/yaml-alias",
	})

	gotYAML, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get yaml metadata returned error: %v", err)
	}
	if gotYAML.IDAttribute != "/yaml-id" || gotYAML.AliasAttribute != "/yaml-alias" {
		t.Fatalf("expected yaml metadata to take precedence, got %+v", gotYAML)
	}
}

func TestFSMetadataResolveForPathWildcardRules(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/_", metadatadomain.ResourceMetadata{
		Transforms: []metadatadomain.TransformStep{
			{ExcludeAttributes: []string{"/root"}},
		},
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
		Transforms: []metadatadomain.TransformStep{
			{ExcludeAttributes: []string{"/wild-1"}},
		},
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
		Transforms: []metadatadomain.TransformStep{},
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

	if len(resolved.Transforms) != 0 {
		t.Fatalf("expected literal array replacement to clear transforms list, got %+v", resolved.Transforms)
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

func TestFSMetadataResolveForPathSecretAttributesLayering(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/_", metadatadomain.ResourceMetadata{
		SecretAttributes: []string{"/credentials/rootSecret"},
	})
	mustSetMetadata(t, service, ctx, "/customers/*", metadatadomain.ResourceMetadata{
		SecretAttributes: []string{"/credentials/wildcardSecret"},
	})
	mustSetMetadata(t, service, ctx, "/customers/acme/_", metadatadomain.ResourceMetadata{
		SecretAttributes: []string{"/credentials/literalSecret"},
	})

	resolved, err := service.ResolveForPath(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("ResolveForPath returned error: %v", err)
	}
	expected := []string{"/credentials/literalSecret"}
	if !reflect.DeepEqual(expected, resolved.SecretAttributes) {
		t.Fatalf("expected literal layer to replace secret attributes, got %#v", resolved.SecretAttributes)
	}
}

func TestFSMetadataResolveForPathIntermediaryPlaceholderSelectors(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_", metadatadomain.ResourceMetadata{
		IDAttribute:    "/realm",
		AliasAttribute: "/realm",
	})
	mustSetMetadata(t, service, ctx, "/admin/realms/_/clients", metadatadomain.ResourceMetadata{
		IDAttribute:    "/id",
		AliasAttribute: "/clientId",
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
	if resolved.IDAttribute != "/id" {
		t.Fatalf("expected clients idAttribute from intermediary placeholder metadata, got %q", resolved.IDAttribute)
	}
	if resolved.AliasAttribute != "/clientId" {
		t.Fatalf(
			"expected clients aliasAttribute from intermediary placeholder metadata, got %q",
			resolved.AliasAttribute,
		)
	}
	createPath := resolved.Operations[string(metadatadomain.OperationCreate)].Path
	if createPath != "/admin/realms/{{.realm}}/clients" {
		t.Fatalf("expected create path from clients metadata, got %q", createPath)
	}
}

func TestFSMetadataResolveCollectionChildrenSupportsIntermediarySelectors(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry/_", metadatadomain.ResourceMetadata{
		IDAttribute:          "/id",
		AliasAttribute:       "/name",
		RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
	})
	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry/_/mappers/_", metadatadomain.ResourceMetadata{
		IDAttribute:          "/id",
		AliasAttribute:       "/name",
		RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
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

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/authentication/flows/_/executions/_", metadatadomain.ResourceMetadata{
		IDAttribute:    "/id",
		AliasAttribute: "/displayName",
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

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		IDAttribute:    "/id",
		AliasAttribute: "/slug",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path: "/api{{.remoteCollectionPath}}/{{.alias}}/{{.remoteID}}",
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

func TestFSMetadataRenderOperationSpecSupportsRemoteCollectionPathIndirection(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/admin/realms/_/user-registry", metadatadomain.ResourceMetadata{
		IDAttribute:          "/id",
		RemoteCollectionPath: "/admin/realms/{{.realm}}/components",
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

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		IDAttribute:    "/id",
		AliasAttribute: "/id",
		PayloadType:    "yaml",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path:   "/api/customers/{{.id}}",
				Accept: "{{payload_media_type .}}",
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

	service := NewFSMetadataService(t.TempDir())
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

func TestFSMetadataValidationStructuredOnlyFields(t *testing.T) {
	t.Parallel()

	wholeSecret := true

	testCases := []struct {
		name    string
		meta    metadatadomain.ResourceMetadata
		want    string
		wantAll []string
	}{
		{
			name: "id_from_attribute_requires_structured_payload",
			meta: metadatadomain.ResourceMetadata{
				PayloadType: "text",
				IDAttribute: "/id",
			},
			want: "resource.idAttribute requires structured payload type (json, yaml)",
		},
		{
			name: "alias_from_attribute_requires_structured_payload",
			meta: metadatadomain.ResourceMetadata{
				PayloadType:    "text",
				AliasAttribute: "/name",
			},
			want: "resource.aliasAttribute requires structured payload type (json, yaml)",
		},
		{
			name: "secret_in_attributes_requires_structured_payload",
			meta: metadatadomain.ResourceMetadata{
				PayloadType:      "text",
				SecretAttributes: []string{"/password"},
			},
			wantAll: []string{
				"resource.secretAttributes requires structured payload type (json, yaml)",
				"resource.secret: true",
			},
		},
		{
			name: "required_attributes_require_structured_payload",
			meta: metadatadomain.ResourceMetadata{
				PayloadType:        "text",
				RequiredAttributes: []string{"/name"},
			},
			want: "resource.requiredAttributes requires structured payload type (json, yaml)",
		},
		{
			name: "externalized_attributes_requires_structured_payload",
			meta: metadatadomain.ResourceMetadata{
				PayloadType: "text",
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{
					{Path: "/script", File: "script.sh"},
				},
			},
			want: "resource.externalizedAttributes requires structured payload type (json, yaml)",
		},
		{
			name: "whole_resource_secret_and_secret_attributes_are_mutually_exclusive",
			meta: metadatadomain.ResourceMetadata{
				Secret:           &wholeSecret,
				SecretAttributes: []string{"/password"},
			},
			want: "resource.secret: true and resource.secretAttributes are mutually exclusive",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := NewFSMetadataService(t.TempDir())
			ctx := context.Background()
			err := service.Set(ctx, "/secrets/app", tt.meta)
			assertTypedCategory(t, err, faults.ValidationError)
			if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got %q", tt.want, err.Error())
			}
			for _, fragment := range tt.wantAll {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("expected error to contain %q, got %q", fragment, err.Error())
				}
			}
		})
	}
}

func TestFSMetadataValidationRejectsInvalidResourceRequiredAttributes(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir())
	ctx := context.Background()

	err := service.Set(ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		RequiredAttributes: []string{"name"},
	})
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "resource.requiredAttributes[0] must be a valid JSON pointer") {
		t.Fatalf("expected resource.requiredAttributes pointer validation error, got %v", err)
	}
}

func TestFSMetadataSetOmitsNilFieldsFromStoredYAML(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	metadata := metadatadomain.ResourceMetadata{
		IDAttribute:      "/id",
		AliasAttribute:   "/clientId",
		SecretAttributes: []string{"/secret"},
	}

	if err := service.Set(ctx, "/admin/realms/_/clients/_", metadata); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	filePath := filepath.Join(baseDir, "admin", "realms", "_", "clients", "_", "metadata.yaml")
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
	if err := yaml.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("failed to decode metadata file: %v", err)
	}

	resource, hasResourceInfo := decoded["resource"].(map[string]any)
	if !hasResourceInfo {
		t.Fatalf("expected resource object, got %#v", decoded["resource"])
	}
	if _, found := resource["secretAttributes"]; !found {
		t.Fatalf("expected secretAttributes under resource, got %#v", resource)
	}
	if _, found := decoded["operations"]; found {
		t.Fatalf("expected operations key to be omitted when nil, got %v", decoded["operations"])
	}
}

func TestFSMetadataSetPreservesExplicitEmptyCollectionsInYAML(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	metadata := metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path:    "/api/customers/{{.id}}",
				Query:   map[string]string{},
				Headers: map[string]string{},
				Transforms: []metadatadomain.TransformStep{
					{SelectAttributes: []string{}},
					{ExcludeAttributes: []string{}},
				},
			},
		},
		Transforms: []metadatadomain.TransformStep{
			{SelectAttributes: []string{}},
			{ExcludeAttributes: []string{}},
		},
	}

	if err := service.Set(ctx, "/customers/acme", metadata); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	filePath := filepath.Join(baseDir, "customers", "acme", "metadata.yaml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read metadata file %q: %v", filePath, err)
	}

	decoded := map[string]any{}
	if err := yaml.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("failed to decode metadata file: %v", err)
	}

	operations, hasOperationsInfo := decoded["operations"].(map[string]any)
	if !hasOperationsInfo {
		t.Fatalf("expected operations object, got %#v", decoded["operations"])
	}

	defaults, hasDefaults := operations["defaults"].(map[string]any)
	if !hasDefaults {
		t.Fatalf("expected operations defaults object, got %#v", operations["defaults"])
	}
	defaultPayload, hasDefaultPayload := defaults["transforms"].([]any)
	if !hasDefaultPayload {
		t.Fatalf("expected operations.defaults.transforms array, got %#v", defaults["transforms"])
	}

	if len(defaultPayload) != 2 {
		t.Fatalf("expected two default transforms steps, got %#v", defaultPayload)
	}
	filterStep, ok := defaultPayload[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first transforms step object, got %#v", defaultPayload[0])
	}
	filter, hasFilter := filterStep["selectAttributes"].([]any)
	if !hasFilter || len(filter) != 0 {
		t.Fatalf("expected explicit empty select array, got %#v", filterStep["selectAttributes"])
	}
	suppressStep, ok := defaultPayload[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second transforms step object, got %#v", defaultPayload[1])
	}
	suppress, hasSuppress := suppressStep["excludeAttributes"].([]any)
	if !hasSuppress || len(suppress) != 0 {
		t.Fatalf("expected explicit empty suppress array, got %#v", suppressStep["excludeAttributes"])
	}

	getSpec, hasGet := operations["get"].(map[string]any)
	if !hasGet {
		t.Fatalf("expected get operation metadata, got %#v", operations["get"])
	}

	query, hasQuery := getSpec["query"].(map[string]any)
	if !hasQuery || len(query) != 0 {
		t.Fatalf("expected explicit empty query object, got %#v", getSpec["query"])
	}
	headers, hasHeaders := getSpec["headers"].(map[string]any)
	if !hasHeaders || len(headers) != 0 {
		t.Fatalf("expected explicit empty headers object, got %#v", getSpec["headers"])
	}
	payload, hasPayload := getSpec["transforms"].([]any)
	if !hasPayload {
		t.Fatalf("expected get operation transforms array, got %#v", getSpec["transforms"])
	}
	if len(payload) != 2 {
		t.Fatalf("expected two operation transforms steps, got %#v", payload)
	}
	specFilterStep, ok := payload[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first operation transforms step object, got %#v", payload[0])
	}
	specFilter, hasSpecFilter := specFilterStep["selectAttributes"].([]any)
	if !hasSpecFilter || len(specFilter) != 0 {
		t.Fatalf("expected explicit empty operation select array, got %#v", specFilterStep["selectAttributes"])
	}
	specSuppressStep, ok := payload[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second operation transforms step object, got %#v", payload[1])
	}
	specSuppress, hasSpecSuppress := specSuppressStep["excludeAttributes"].([]any)
	if !hasSpecSuppress || len(specSuppress) != 0 {
		t.Fatalf("expected explicit empty operation suppress array, got %#v", specSuppressStep["excludeAttributes"])
	}
}

func TestFSMetadataSetWritesYAMLAndRemovesExistingJSON(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	jsonPath := filepath.Join(baseDir, "customers", "acme", "metadata.json")
	writeMetadataFixture(t, jsonPath, false, metadatadomain.ResourceMetadata{
		IDAttribute:    "/legacy-id",
		AliasAttribute: "/legacy-alias",
	})

	updated := metadatadomain.ResourceMetadata{
		IDAttribute:    "/yaml-id",
		AliasAttribute: "/yaml-alias",
	}
	if err := service.Set(ctx, "/customers/acme", updated); err != nil {
		t.Fatalf("Set metadata returned error: %v", err)
	}

	yamlPath := filepath.Join(baseDir, "customers", "acme", "metadata.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("expected yaml metadata file %q: %v", yamlPath, err)
	}
	if _, err := os.Stat(jsonPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected json metadata file to be removed, got err=%v", err)
	}
}

func TestFSMetadataGetRejectsOperationURLPathSyntax(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	service := NewFSMetadataService(baseDir)
	ctx := context.Background()

	filePath := filepath.Join(baseDir, "admin", "realms", "_", "user-registry", "_", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create metadata directory %q: %v", filepath.Dir(filePath), err)
	}

	payload := `{
  "resource": {
    "remoteCollectionPath": "/admin/realms/{{.realm}}/components"
  },
  "operations": {
    "get": {
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

	if _, err := service.Get(ctx, "/admin/realms/_/user-registry"); err == nil {
		t.Fatal("expected Get to reject operations.get.url.path")
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

func writeMetadataFixture(
	t *testing.T,
	filePath string,
	useYAML bool,
	metadata metadatadomain.ResourceMetadata,
) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("failed to create metadata directory %q: %v", filepath.Dir(filePath), err)
	}

	var (
		encoded []byte
		err     error
	)
	if useYAML {
		encoded, err = metadatadomain.EncodeResourceMetadataYAML(metadata)
	} else {
		encoded, err = metadatadomain.EncodeResourceMetadataJSON(metadata, true)
	}
	if err != nil {
		t.Fatalf("failed to encode metadata fixture %q: %v", filePath, err)
	}

	if err := os.WriteFile(filePath, ensureTrailingNewline(encoded), 0o644); err != nil {
		t.Fatalf("failed to write metadata fixture %q: %v", filePath, err)
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
