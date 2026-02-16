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
