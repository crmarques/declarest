package file

import (
	"context"
	"encoding/json"
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

func TestFileSecretServiceKDFEnvelopeParams(t *testing.T) {
	t.Parallel()

	t.Run("new_store_embeds_kdf_params", func(t *testing.T) {
		t.Parallel()

		secretFilePath := filepath.Join(t.TempDir(), "secrets.enc")
		service, err := NewFileSecretService(config.FileSecretStore{
			Path:       secretFilePath,
			Passphrase: "change-me",
		})
		if err != nil {
			t.Fatalf("NewFileSecretService returned error: %v", err)
		}

		if err := service.Store(context.Background(), "key1", "val1"); err != nil {
			t.Fatalf("Store returned error: %v", err)
		}

		data, err := os.ReadFile(secretFilePath)
		if err != nil {
			t.Fatalf("failed to read secret file: %v", err)
		}

		var envelope encryptedStore
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("failed to unmarshal envelope: %v", err)
		}

		if envelope.KDFTime != defaultKDFTime {
			t.Fatalf("expected KDFTime %d, got %d", defaultKDFTime, envelope.KDFTime)
		}
		if envelope.KDFMemory != defaultKDFMemory {
			t.Fatalf("expected KDFMemory %d, got %d", defaultKDFMemory, envelope.KDFMemory)
		}
		if envelope.KDFThreads != defaultKDFThreads {
			t.Fatalf("expected KDFThreads %d, got %d", defaultKDFThreads, envelope.KDFThreads)
		}
	})

	t.Run("legacy_store_without_kdf_fields_still_decrypts", func(t *testing.T) {
		t.Parallel()

		// Create a store with legacy KDF settings (Time=1) by directly constructing a service.
		secretFilePath := filepath.Join(t.TempDir(), "secrets.enc")
		legacyService := &FileSecretService{
			path:       secretFilePath,
			passphrase: []byte("change-me"),
			kdf:        kdfSettings{Time: 1, Memory: 64 * 1024, Threads: 4},
		}

		if err := legacyService.Store(context.Background(), "legacyKey", "legacyValue"); err != nil {
			t.Fatalf("legacy Store returned error: %v", err)
		}

		// Manually strip KDF fields to simulate a pre-migration envelope.
		data, err := os.ReadFile(secretFilePath)
		if err != nil {
			t.Fatalf("failed to read secret file: %v", err)
		}
		var envelope encryptedStore
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("failed to unmarshal envelope: %v", err)
		}
		envelope.KDFTime = 0
		envelope.KDFMemory = 0
		envelope.KDFThreads = 0
		strippedData, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("failed to marshal stripped envelope: %v", err)
		}
		if err := os.WriteFile(secretFilePath, strippedData, 0o600); err != nil {
			t.Fatalf("failed to write stripped envelope: %v", err)
		}

		// Now read with a new service using current defaults (Time=3).
		newService, err := NewFileSecretService(config.FileSecretStore{
			Path:       secretFilePath,
			Passphrase: "change-me",
		})
		if err != nil {
			t.Fatalf("NewFileSecretService returned error: %v", err)
		}

		val, err := newService.Get(context.Background(), "legacyKey")
		if err != nil {
			t.Fatalf("Get returned error (legacy compat failed): %v", err)
		}
		if val != "legacyValue" {
			t.Fatalf("expected legacyValue, got %q", val)
		}
	})

	t.Run("migration_on_rewrite_embeds_new_kdf_params", func(t *testing.T) {
		t.Parallel()

		// Create a legacy store (Time=1, no KDF fields).
		secretFilePath := filepath.Join(t.TempDir(), "secrets.enc")
		legacyService := &FileSecretService{
			path:       secretFilePath,
			passphrase: []byte("change-me"),
			kdf:        kdfSettings{Time: 1, Memory: 64 * 1024, Threads: 4},
		}

		if err := legacyService.Store(context.Background(), "key1", "val1"); err != nil {
			t.Fatalf("legacy Store returned error: %v", err)
		}

		// Strip KDF fields to simulate legacy format.
		data, err := os.ReadFile(secretFilePath)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		var envelope encryptedStore
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		envelope.KDFTime = 0
		envelope.KDFMemory = 0
		envelope.KDFThreads = 0
		strippedData, err := json.Marshal(envelope)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		if err := os.WriteFile(secretFilePath, strippedData, 0o600); err != nil {
			t.Fatalf("failed to write: %v", err)
		}

		// Open with new service and write a new key — triggers re-encryption with new KDF.
		newService, err := NewFileSecretService(config.FileSecretStore{
			Path:       secretFilePath,
			Passphrase: "change-me",
		})
		if err != nil {
			t.Fatalf("NewFileSecretService returned error: %v", err)
		}

		if err := newService.Store(context.Background(), "key2", "val2"); err != nil {
			t.Fatalf("Store returned error: %v", err)
		}

		// Verify the envelope now has the new KDF params.
		data, err = os.ReadFile(secretFilePath)
		if err != nil {
			t.Fatalf("failed to read migrated file: %v", err)
		}
		var migratedEnvelope encryptedStore
		if err := json.Unmarshal(data, &migratedEnvelope); err != nil {
			t.Fatalf("failed to unmarshal migrated envelope: %v", err)
		}
		if migratedEnvelope.KDFTime != defaultKDFTime {
			t.Fatalf("expected migrated KDFTime %d, got %d", defaultKDFTime, migratedEnvelope.KDFTime)
		}
		if migratedEnvelope.KDFMemory != defaultKDFMemory {
			t.Fatalf("expected migrated KDFMemory %d, got %d", defaultKDFMemory, migratedEnvelope.KDFMemory)
		}

		// Verify both keys are accessible.
		val1, err := newService.Get(context.Background(), "key1")
		if err != nil {
			t.Fatalf("Get key1 returned error: %v", err)
		}
		if val1 != "val1" {
			t.Fatalf("expected val1, got %q", val1)
		}
		val2, err := newService.Get(context.Background(), "key2")
		if err != nil {
			t.Fatalf("Get key2 returned error: %v", err)
		}
		if val2 != "val2" {
			t.Fatalf("expected val2, got %q", val2)
		}
	})

	t.Run("key_based_store_has_no_kdf_fields", func(t *testing.T) {
		t.Parallel()

		secretFilePath := filepath.Join(t.TempDir(), "secrets.enc")
		service, err := NewFileSecretService(config.FileSecretStore{
			Path: secretFilePath,
			Key:  "0123456789abcdef0123456789abcdef",
		})
		if err != nil {
			t.Fatalf("NewFileSecretService returned error: %v", err)
		}

		if err := service.Store(context.Background(), "key1", "val1"); err != nil {
			t.Fatalf("Store returned error: %v", err)
		}

		data, err := os.ReadFile(secretFilePath)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		var envelope encryptedStore
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if envelope.KDFTime != 0 || envelope.KDFMemory != 0 || envelope.KDFThreads != 0 {
			t.Fatalf("key-based store should not have KDF fields, got time=%d memory=%d threads=%d",
				envelope.KDFTime, envelope.KDFMemory, envelope.KDFThreads)
		}
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
