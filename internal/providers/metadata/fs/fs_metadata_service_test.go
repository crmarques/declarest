package fsmetadata

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

	writeRawMetadataFile(t, filepath.Join(baseDir, "admin", "realms", "_", "metadata.json"), metadatadomain.ResourceMetadata{
		IDFromAttribute:    "realm",
		AliasFromAttribute: "realm",
	})
	writeRawMetadataFile(t, filepath.Join(baseDir, "admin", "realms", "_", "clients", "_", "metadata.json"), metadatadomain.ResourceMetadata{
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

func TestFSMetadataInferPreservesExplicitAndApply(t *testing.T) {
	t.Parallel()

	service := NewFSMetadataService(t.TempDir(), "")
	ctx := context.Background()

	mustSetMetadata(t, service, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet): {
				Path:   "/custom/get/path",
				Method: "GET",
			},
		},
	})

	inferredPreview, err := service.Infer(ctx, "/customers/acme", metadatadomain.InferenceRequest{Apply: false})
	if err != nil {
		t.Fatalf("Infer preview returned error: %v", err)
	}
	if inferredPreview.Operations[string(metadatadomain.OperationGet)].Path != "/custom/get/path" {
		t.Fatalf("expected explicit get path to win, got %+v", inferredPreview.Operations[string(metadatadomain.OperationGet)])
	}
	if inferredPreview.Operations[string(metadatadomain.OperationUpdate)].Path == "" {
		t.Fatalf("expected inferred update operation to be present, got %+v", inferredPreview.Operations)
	}

	stillStored, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get after preview infer returned error: %v", err)
	}
	if stillStored.Operations[string(metadatadomain.OperationUpdate)].Path != "" {
		t.Fatalf("expected preview infer to avoid persistence, got %+v", stillStored.Operations)
	}

	_, err = service.Infer(ctx, "/customers/acme", metadatadomain.InferenceRequest{Apply: true})
	if err != nil {
		t.Fatalf("Infer apply returned error: %v", err)
	}

	stored, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get after apply infer returned error: %v", err)
	}
	if stored.Operations[string(metadatadomain.OperationGet)].Path != "/custom/get/path" {
		t.Fatalf("expected explicit get path to remain, got %+v", stored.Operations[string(metadatadomain.OperationGet)])
	}
	if stored.Operations[string(metadatadomain.OperationUpdate)].Path == "" {
		t.Fatalf("expected inferred update operation to be persisted, got %+v", stored.Operations)
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
