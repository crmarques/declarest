package resource

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
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

func TestDetectSaveSecretCandidates(t *testing.T) {
	t.Parallel()

	t.Run("metadata_secrets_from_attributes_detects_plaintext", func(t *testing.T) {
		t.Parallel()

		deps := common.CommandDependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"credentials": map[string]any{"authValue": "plain-secret"},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}

		if len(candidates) != 1 || candidates[0] != "credentials.authValue" {
			t.Fatalf("expected metadata candidate, got %#v", candidates)
		}
	})

	t.Run("metadata_secrets_from_attributes_ignores_placeholders", func(t *testing.T) {
		t.Parallel()

		deps := common.CommandDependencies{
			Metadata: &fakeSaveMetadataService{
				resolved: metadatadomain.ResourceMetadata{
					SecretsFromAttributes: []string{"credentials.authValue"},
				},
			},
			Secrets: &fakeSaveSecretProvider{},
		}

		candidates, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"credentials": map[string]any{"authValue": `{{secret "authValue"}}`},
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("expected no candidates for placeholder, got %#v", candidates)
		}
	})

	t.Run("falls_back_to_builtin_detection_without_secret_provider", func(t *testing.T) {
		t.Parallel()

		candidates, err := detectSaveSecretCandidates(context.Background(), common.CommandDependencies{}, "/customers/acme", map[string]any{
			"password": "plain-secret",
		})
		if err != nil {
			t.Fatalf("detectSaveSecretCandidates returned error: %v", err)
		}
		if len(candidates) != 1 || candidates[0] != "password" {
			t.Fatalf("expected password candidate, got %#v", candidates)
		}
	})

	t.Run("secret_provider_error_is_returned", func(t *testing.T) {
		t.Parallel()

		expectedErr := faults.NewTypedError(faults.TransportError, "detect failed", nil)
		deps := common.CommandDependencies{
			Secrets: &fakeSaveSecretProvider{detectErr: expectedErr},
		}

		_, err := detectSaveSecretCandidates(context.Background(), deps, "/customers/acme", map[string]any{
			"password": "plain-secret",
		})
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected detect error %v, got %v", expectedErr, err)
		}
	})
}

func TestEnforceSaveSecretSafety(t *testing.T) {
	t.Parallel()

	t.Run("fails_without_ignore_when_plaintext_secret_detected", func(t *testing.T) {
		t.Parallel()

		err := enforceSaveSecretSafety(
			context.Background(),
			common.CommandDependencies{},
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
			false,
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "warning: potential plaintext secrets detected") {
			t.Fatalf("expected warning in error message, got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "--ignore") {
			t.Fatalf("expected --ignore hint in error message, got %q", err.Error())
		}
	})

	t.Run("allows_plaintext_secret_when_ignore_is_enabled", func(t *testing.T) {
		t.Parallel()

		err := enforceSaveSecretSafety(
			context.Background(),
			common.CommandDependencies{},
			"/customers/acme",
			map[string]any{"password": "plain-secret"},
			true,
		)
		if err != nil {
			t.Fatalf("enforceSaveSecretSafety returned error: %v", err)
		}
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

type fakeSaveSecretProvider struct {
	detectedCandidates []string
	detectErr          error
}

func (f *fakeSaveSecretProvider) Init(context.Context) error { return nil }
func (f *fakeSaveSecretProvider) Store(context.Context, string, string) error {
	return nil
}
func (f *fakeSaveSecretProvider) Get(context.Context, string) (string, error) { return "", nil }
func (f *fakeSaveSecretProvider) Delete(context.Context, string) error        { return nil }
func (f *fakeSaveSecretProvider) List(context.Context) ([]string, error)      { return nil, nil }
func (f *fakeSaveSecretProvider) MaskPayload(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeSaveSecretProvider) ResolvePayload(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeSaveSecretProvider) NormalizeSecretPlaceholders(context.Context, resourcedomain.Value) (resourcedomain.Value, error) {
	return nil, nil
}
func (f *fakeSaveSecretProvider) DetectSecretCandidates(context.Context, resourcedomain.Value) ([]string, error) {
	if f.detectErr != nil {
		return nil, f.detectErr
	}
	return f.detectedCandidates, nil
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
