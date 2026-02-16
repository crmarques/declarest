package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func TestExtractSaveListItems(t *testing.T) {
	t.Parallel()

	t.Run("array_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems([]any{
			map[string]any{"id": "a"},
			map[string]any{"id": "b"},
		})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if !isList {
			t.Fatal("expected list payload to be detected")
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("items_object_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems(map[string]any{
			"items": []any{
				map[string]any{"id": "a"},
			},
		})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if !isList {
			t.Fatal("expected object with items to be detected as list payload")
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
	})

	t.Run("items_object_invalid_shape", func(t *testing.T) {
		t.Parallel()

		_, _, err := extractSaveListItems(map[string]any{
			"items": map[string]any{"id": "a"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("non_list_payload", func(t *testing.T) {
		t.Parallel()

		items, isList, err := extractSaveListItems(map[string]any{"id": "a"})
		if err != nil {
			t.Fatalf("extractSaveListItems returned error: %v", err)
		}
		if isList {
			t.Fatal("expected non-list payload to not be detected as list")
		}
		if items != nil {
			t.Fatalf("expected nil items, got %#v", items)
		}
	})
}

func TestResolveSaveEntriesForItems(t *testing.T) {
	t.Parallel()

	t.Run("metadata_alias_resolution_and_deterministic_order", func(t *testing.T) {
		t.Parallel()

		deps := common.CommandDependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{AliasFromAttribute: "alias"},
			},
		}

		entries, err := resolveSaveEntriesForItems(context.Background(), deps, "/customers", []any{
			map[string]any{"alias": "zeta", "tier": "pro"},
			map[string]any{"alias": "alpha", "tier": "free"},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/customers/alpha" || entries[1].LogicalPath != "/customers/zeta" {
			t.Fatalf("expected deterministic sorted paths, got %#v", entries)
		}
	})

	t.Run("resource_entry_shape_bypasses_metadata", func(t *testing.T) {
		t.Parallel()

		entries, err := resolveSaveEntriesForItems(context.Background(), common.CommandDependencies{}, "/ignored", []any{
			map[string]any{"LogicalPath": "/customers/zeta", "Payload": map[string]any{"id": "zeta"}},
			map[string]any{"LogicalPath": "/customers/alpha", "Payload": map[string]any{"id": "alpha"}},
		})
		if err != nil {
			t.Fatalf("resolveSaveEntriesForItems returned error: %v", err)
		}

		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].LogicalPath != "/customers/alpha" || entries[1].LogicalPath != "/customers/zeta" {
			t.Fatalf("expected deterministic sorted paths, got %#v", entries)
		}
	})

	t.Run("duplicate_resolved_path_fails", func(t *testing.T) {
		t.Parallel()

		deps := common.CommandDependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{AliasFromAttribute: "alias"},
			},
		}

		_, err := resolveSaveEntriesForItems(context.Background(), deps, "/customers", []any{
			map[string]any{"alias": "dup"},
			map[string]any{"alias": "dup"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("resource_entry_shape_missing_payload_fails", func(t *testing.T) {
		t.Parallel()

		_, err := resolveSaveEntriesForItems(context.Background(), common.CommandDependencies{}, "/customers", []any{
			map[string]any{"LogicalPath": "/customers/acme"},
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestIsTypedErrorCategory(t *testing.T) {
	t.Parallel()

	if isTypedErrorCategory(nil, faults.ValidationError) {
		t.Fatal("expected nil error to not match typed category")
	}

	if isTypedErrorCategory(errors.New("plain error"), faults.ValidationError) {
		t.Fatal("expected non-typed error to not match typed category")
	}

	typedErr := faults.NewTypedError(faults.ValidationError, "bad input", nil)
	if !isTypedErrorCategory(typedErr, faults.ValidationError) {
		t.Fatal("expected typed error to match category")
	}
	if isTypedErrorCategory(typedErr, faults.NotFoundError) {
		t.Fatal("expected typed error to not match different category")
	}
}

type fakeSaveMetadataService struct {
	resolved   metadatadomain.ResourceMetadata
	resolveErr error
}

func (f *fakeSaveMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (f *fakeSaveMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f *fakeSaveMetadataService) Unset(context.Context, string) error { return nil }

func (f *fakeSaveMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	if f.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, f.resolveErr
	}
	return f.resolved, nil
}

func (f *fakeSaveMetadataService) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

func (f *fakeSaveMetadataService) Infer(
	context.Context,
	string,
	metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
