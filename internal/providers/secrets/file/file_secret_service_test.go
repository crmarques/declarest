package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestFileSecretServiceCRUD(t *testing.T) {
	t.Parallel()

	secretFilePath := filepath.Join(t.TempDir(), "secrets.enc")
	service, err := NewFileSecretService(config.FileSecretStore{
		Path:       secretFilePath,
		Passphrase: "change-me",
	})
	if err != nil {
		t.Fatalf("NewFileSecretService returned error: %v", err)
	}

	if err := service.Init(context.Background()); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if err := service.Store(context.Background(), "apiToken", "top-secret"); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	value, err := service.Get(context.Background(), "apiToken")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "top-secret" {
		t.Fatalf("expected top-secret, got %q", value)
	}

	keys, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !reflect.DeepEqual(keys, []string{"apiToken"}) {
		t.Fatalf("expected [apiToken], got %#v", keys)
	}

	encoded, err := os.ReadFile(secretFilePath)
	if err != nil {
		t.Fatalf("failed to read encrypted file: %v", err)
	}
	if strings.Contains(string(encoded), "top-secret") {
		t.Fatal("secret file contains plaintext secret")
	}

	if err := service.Delete(context.Background(), "apiToken"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	_, err = service.Get(context.Background(), "apiToken")
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestFileSecretServicePayloadOperations(t *testing.T) {
	t.Parallel()

	service, err := NewFileSecretService(config.FileSecretStore{
		Path:       filepath.Join(t.TempDir(), "secrets.enc"),
		Passphrase: "change-me",
	})
	if err != nil {
		t.Fatalf("NewFileSecretService returned error: %v", err)
	}

	input := map[string]any{
		"name":     "acme",
		"apiToken": "token-1",
	}

	masked, err := service.MaskPayload(context.Background(), input)
	if err != nil {
		t.Fatalf("MaskPayload returned error: %v", err)
	}
	expectedMasked := map[string]any{
		"name":     "acme",
		"apiToken": "{{secret .}}",
	}
	if !reflect.DeepEqual(masked, expectedMasked) {
		t.Fatalf("expected masked %#v, got %#v", expectedMasked, masked)
	}

	resolved, err := service.ResolvePayload(context.Background(), masked)
	if err != nil {
		t.Fatalf("ResolvePayload returned error: %v", err)
	}
	if !reflect.DeepEqual(resolved, input) {
		t.Fatalf("expected resolved %#v, got %#v", input, resolved)
	}

	normalized, err := service.NormalizeSecretPlaceholders(context.Background(), map[string]any{
		"apiToken": "{{ secret . }}",
	})
	if err != nil {
		t.Fatalf("NormalizeSecretPlaceholders returned error: %v", err)
	}
	expectedNormalized := map[string]any{"apiToken": "{{secret .}}"}
	if !reflect.DeepEqual(normalized, expectedNormalized) {
		t.Fatalf("expected normalized %#v, got %#v", expectedNormalized, normalized)
	}

	candidates, err := service.DetectSecretCandidates(context.Background(), input)
	if err != nil {
		t.Fatalf("DetectSecretCandidates returned error: %v", err)
	}
	if !reflect.DeepEqual(candidates, []string{"apiToken"}) {
		t.Fatalf("expected [apiToken], got %#v", candidates)
	}
}

func TestFileSecretServiceValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid_config", func(t *testing.T) {
		t.Parallel()

		_, err := NewFileSecretService(config.FileSecretStore{
			Path: filepath.Join(t.TempDir(), "secrets.enc"),
		})
		assertTypedCategory(t, err, faults.ValidationError)

		_, err = NewFileSecretService(config.FileSecretStore{
			Path: filepath.Join(t.TempDir(), "secrets.enc"),
			Key:  "short",
		})
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("rejects_ambiguous_masking", func(t *testing.T) {
		t.Parallel()

		service, err := NewFileSecretService(config.FileSecretStore{
			Path:       filepath.Join(t.TempDir(), "secrets.enc"),
			Passphrase: "change-me",
		})
		if err != nil {
			t.Fatalf("NewFileSecretService returned error: %v", err)
		}

		input := map[string]any{
			"a": map[string]any{"token": "a"},
			"b": map[string]any{"token": "b"},
		}

		_, err = service.MaskPayload(context.Background(), input)
		assertTypedCategory(t, err, faults.ValidationError)
	})
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
