package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestResourceSaveFilePayloadPreservesOpaqueExtensionAndBytes(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	payloadFile := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(payloadFile, []byte("private-key-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	_, err := executeForTest(
		deps,
		"",
		"resource",
		"save",
		"/projects/platform/secrets/private-key",
		"--payload",
		payloadFile,
		"--overwrite",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orchestrator.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %d", len(orchestrator.saveCalls))
	}
	if orchestrator.saveCalls[0].descriptor.Extension != ".key" {
		t.Fatalf("expected .key extension, got %q", orchestrator.saveCalls[0].descriptor.Extension)
	}
	binaryValue, ok := orchestrator.saveCalls[0].value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected BinaryValue payload, got %T", orchestrator.saveCalls[0].value)
	}
	if string(binaryValue.Bytes) != "private-key-bytes" {
		t.Fatalf("expected original file bytes, got %q", string(binaryValue.Bytes))
	}
}

func TestResourceSaveSecretStoresWholePayloadInSecretStore(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	metadataService.items["/projects/platform/secrets/private-key"] = metadatadomain.ResourceMetadata{
		SecretsFromAttributes: []string{"/password"},
	}
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	payloadFile := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(payloadFile, []byte("private-key-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	_, err := executeForTest(
		deps,
		"",
		"resource",
		"save",
		"/projects/platform/secrets/private-key",
		"--payload",
		payloadFile,
		"--secret",
		"--overwrite",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orchestrator.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %d", len(orchestrator.saveCalls))
	}
	if orchestrator.saveCalls[0].descriptor.Extension != ".key" {
		t.Fatalf("expected .key extension, got %q", orchestrator.saveCalls[0].descriptor.Extension)
	}
	placeholder, ok := orchestrator.saveCalls[0].value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected binary placeholder payload, got %T", orchestrator.saveCalls[0].value)
	}
	if string(placeholder.Bytes) != "{{secret .}}" {
		t.Fatalf("expected whole-resource placeholder bytes, got %q", string(placeholder.Bytes))
	}

	secretProvider := deps.Services.(*testServiceAccessor).secrets.(*testSecretProvider)
	if got := secretProvider.values["/projects/platform/secrets/private-key:."]; got != "private-key-bytes" {
		t.Fatalf("expected whole payload to be stored under root key, got %#v", secretProvider.values)
	}
	savedMetadata := metadataService.items["/projects/platform/secrets/private-key"]
	if !savedMetadata.IsWholeResourceSecret() {
		t.Fatalf("expected metadata secret declaration to be persisted, got %#v", savedMetadata)
	}
	if savedMetadata.SecretsFromAttributes != nil {
		t.Fatalf("expected attribute-level secret metadata to be cleared, got %#v", savedMetadata.SecretsFromAttributes)
	}
}

func TestResourceSaveAutoSecretFromMetadataStoresWholePayloadInSecretStore(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	wholeSecret := true
	metadataService.items["/projects/platform/secrets/private-key"] = metadatadomain.ResourceMetadata{
		Secret: &wholeSecret,
	}
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	payloadFile := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(payloadFile, []byte("private-key-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	_, err := executeForTest(
		deps,
		"",
		"resource",
		"save",
		"/projects/platform/secrets/private-key",
		"--payload",
		payloadFile,
		"--overwrite",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orchestrator.saveCalls) != 1 {
		t.Fatalf("expected one save call, got %d", len(orchestrator.saveCalls))
	}
	placeholder, ok := orchestrator.saveCalls[0].value.(resource.BinaryValue)
	if !ok {
		t.Fatalf("expected binary placeholder payload, got %T", orchestrator.saveCalls[0].value)
	}
	if string(placeholder.Bytes) != "{{secret .}}" {
		t.Fatalf("expected whole-resource placeholder bytes, got %q", string(placeholder.Bytes))
	}

	secretProvider := deps.Services.(*testServiceAccessor).secrets.(*testSecretProvider)
	if got := secretProvider.values["/projects/platform/secrets/private-key:."]; got != "private-key-bytes" {
		t.Fatalf("expected whole payload to be stored under root key, got %#v", secretProvider.values)
	}
}

func TestResourceSaveRejectsMissingPathLikePayload(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	_, err := executeForTest(
		deps,
		"",
		"resource",
		"save",
		"/projects/platform/secrets/private-key",
		"--payload",
		"test/e2e/.runs/20260308-170415-3098387/private.key",
		"--overwrite",
	)
	if err == nil {
		t.Fatal("expected missing payload file error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
	if len(orchestrator.saveCalls) != 0 {
		t.Fatalf("expected save to fail before mutation, got %d save calls", len(orchestrator.saveCalls))
	}
}

func TestResourceSaveSecretRejectsConflictingFlags(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	_, err := executeForTest(
		deps,
		`{"password":"plain-secret"}`,
		"resource",
		"save",
		"/customers/acme",
		"--secret",
		"--secret-attributes",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "--secret") || !strings.Contains(err.Error(), "--secret-attributes") {
		t.Fatalf("expected secret conflict error, got %v", err)
	}
}

func TestResourceSaveSecretAttributesRejectsNonStructuredPayload(t *testing.T) {
	t.Parallel()

	metadataService := newTestMetadata()
	orchestrator := &testOrchestrator{metadataService: metadataService}
	deps := newResourceSaveDeps(orchestrator, metadataService)

	payloadFile := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(payloadFile, []byte("private-key-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write payload file: %v", err)
	}

	_, err := executeForTest(
		deps,
		"",
		"resource",
		"save",
		"/projects/platform/secrets/private-key",
		"--payload",
		payloadFile,
		"--secret-attributes",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "--secret-attributes requires structured payload") || !strings.Contains(err.Error(), "--secret") {
		t.Fatalf("expected structured payload guidance error, got %v", err)
	}
}

func TestResourceSaveHelpIncludesSecretFlag(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "resource", "save", "--help")
	if err != nil {
		t.Fatalf("expected resource save help output, got error: %v", err)
	}
	if !strings.Contains(output, "--secret") {
		t.Fatalf("expected --secret in resource save help output, got %q", output)
	}
}
