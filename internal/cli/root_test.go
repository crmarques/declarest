package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func TestRequiredCommandPathsRegistered(t *testing.T) {
	t.Parallel()

	requiredPaths := []string{
		"ad-hoc",
		"ad-hoc get",
		"ad-hoc post",
		"ad-hoc put",
		"ad-hoc patch",
		"ad-hoc delete",
		"ad-hoc head",
		"ad-hoc options",
		"ad-hoc trace",
		"ad-hoc connect",
		"config",
		"config create",
		"config add",
		"config use",
		"config current",
		"config check",
		"config resolve",
		"resource",
		"resource get",
		"resource delete",
		"metadata",
		"metadata resolve",
		"metadata render",
		"repo",
		"repo status",
		"secret",
		"secret resolve",
		"completion",
		"completion bash",
		"version",
	}

	pathSet := make(map[string]struct{})
	for _, path := range registeredPaths(NewRootCommand(testDeps()), nil) {
		pathSet[joinPath(path)] = struct{}{}
	}

	for _, required := range requiredPaths {
		if _, ok := pathSet[required]; !ok {
			t.Fatalf("expected command path %q to be registered", required)
		}
	}
}

func TestLegacyCommandNamesRemoved(t *testing.T) {
	t.Parallel()

	legacyPaths := []string{
		"config set-current",
		"config get-current",
		"config load-resolved-config",
		"metadata resolve-for-path",
		"metadata render-operation-spec",
		"repo sync-status",
		"secret mask-payload",
		"generic",
	}

	pathSet := make(map[string]struct{})
	for _, path := range registeredPaths(NewRootCommand(testDeps()), nil) {
		pathSet[joinPath(path)] = struct{}{}
	}

	for _, legacyPath := range legacyPaths {
		if _, ok := pathSet[legacyPath]; ok {
			t.Fatalf("expected command path %q to be removed", legacyPath)
		}
	}
}

func TestRootWithoutArgsShowsHelp(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "")
	if err != nil {
		t.Fatalf("root command returned error: %v", err)
	}
	if !strings.Contains(output, "Basic Commands:") {
		t.Fatalf("expected grouped help output, got %q", output)
	}
}

func TestGlobalFlagsParse(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "-c", "prod", "-d", "-n", "-o", "json", "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "\"version\"") {
		t.Fatalf("expected json version output, got %q", output)
	}
}

func TestDebugFlagPrintsTraceOutput(t *testing.T) {
	t.Parallel()

	output, debugOutput, err := executeForTestWithStreams(testDeps(), "", "--debug", "resource", "get", "/customers/acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "/customers/acme") {
		t.Fatalf("expected output to contain path, got %q", output)
	}
	if !strings.Contains(debugOutput, `debug: root flags context="" output="auto" no_status=false command="declarest resource get"`) {
		t.Fatalf("expected root debug trace, got %q", debugOutput)
	}
	if !strings.Contains(debugOutput, `debug: resource get requested path="/customers/acme"`) {
		t.Fatalf("expected resource get debug trace, got %q", debugOutput)
	}
	if !strings.Contains(debugOutput, `debug: resource get succeeded path="/customers/acme" value_type=map[string]interface {}`) {
		t.Fatalf("expected resource get success debug trace, got %q", debugOutput)
	}
}

func TestMetadataDebugTraceIncludesLookupPath(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	metadataService := fsmetadata.NewFSMetadataService(baseDir, config.ResourceFormatJSON)
	if err := metadataService.Set(context.Background(), "/admin/realms/_", metadatadomain.ResourceMetadata{
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationList): {Path: "/api/admin/realms"},
		},
	}); err != nil {
		t.Fatalf("failed to seed metadata fixture: %v", err)
	}

	deps := Dependencies{
		Contexts: &testContextService{},
		Metadata: metadataService,
	}

	output, debugOutput, err := executeForTestWithStreams(deps, "", "--debug", "metadata", "get", "/admin/realms/")
	if err != nil {
		t.Fatalf("unexpected metadata get error: %v", err)
	}
	if !strings.Contains(output, "\"path\": \"/api/admin/realms\"") {
		t.Fatalf("expected metadata get output payload, got %q", output)
	}

	expectedMetadataPath := filepath.Join(baseDir, "admin", "realms", "_", "metadata.json")
	if !strings.Contains(debugOutput, `debug: metadata get requested path="/admin/realms/"`) {
		t.Fatalf("expected metadata get debug trace, got %q", debugOutput)
	}
	if !strings.Contains(debugOutput, `debug: metadata fs get lookup logical_path="/admin/realms/" selector="/admin/realms" kind="collection"`) {
		t.Fatalf("expected metadata filesystem lookup trace, got %q", debugOutput)
	}
	if !strings.Contains(debugOutput, expectedMetadataPath) {
		t.Fatalf("expected metadata lookup path %q in debug output, got %q", expectedMetadataPath, debugOutput)
	}
}

func TestResourceGetDualPathInput(t *testing.T) {
	t.Parallel()

	t.Run("positional_path", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "/customers/acme") {
			t.Fatalf("expected output to contain path, got %q", output)
		}
	})

	t.Run("flag_path", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "--path", "/customers/beta")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "/customers/beta") {
			t.Fatalf("expected output to contain path, got %q", output)
		}
	})

	t.Run("matching_path_inputs", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/gamma", "--path", "/customers/gamma")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("mismatch_path_inputs", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/a", "--path", "/customers/b")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("missing_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceGetSourceSelection(t *testing.T) {
	t.Parallel()

	t.Run("default_uses_remote", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"remote\"") {
			t.Fatalf("expected remote source output by default, got %q", output)
		}
	})

	t.Run("repository_flag_uses_repository", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--repository")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"local\"") {
			t.Fatalf("expected repository source output, got %q", output)
		}
	})

	t.Run("remote_server_flag_uses_remote", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--remote-server")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"remote\"") {
			t.Fatalf("expected remote source output, got %q", output)
		}
	})

	t.Run("repository_and_remote_server_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestAdHocMethodCommands(t *testing.T) {
	t.Parallel()

	t.Run("get_positional_path", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "get", "/test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"GET\"") {
			t.Fatalf("expected GET method output, got %q", output)
		}
		if !strings.Contains(output, "\"path\": \"/test\"") {
			t.Fatalf("expected path output, got %q", output)
		}
	})

	t.Run("get_not_found_falls_back_to_metadata_aware_resource_get", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		reconciler := deps.Orchestrator.(*testReconciler)
		reconciler.adHocErr = faults.NewTypedError(faults.NotFoundError, "ad-hoc path not found", nil)
		reconciler.getRemoteValue = map[string]any{
			"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
			"clientId": "account",
		}

		output, err := executeForTest(deps, "", "ad-hoc", "get", "/admin/realms/master/clients/account")
		if err != nil {
			t.Fatalf("unexpected fallback error: %v", err)
		}
		if len(reconciler.getRemoteCalls) != 1 {
			t.Fatalf("expected one metadata-aware fallback read, got %d", len(reconciler.getRemoteCalls))
		}
		if reconciler.getRemoteCalls[0] != "/admin/realms/master/clients/account" {
			t.Fatalf("expected fallback path to match request, got %q", reconciler.getRemoteCalls[0])
		}
		if !strings.Contains(output, "\"clientId\": \"account\"") {
			t.Fatalf("expected fallback payload output, got %q", output)
		}
	})

	t.Run("post_reads_stdin_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), `{"id":"a","name":"alpha"}`, "ad-hoc", "post", "/items")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"POST\"") {
			t.Fatalf("expected POST method output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"alpha\"") {
			t.Fatalf("expected stdin payload to be forwarded, got %q", output)
		}
	})

	t.Run("put_reads_file_body", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		payloadPath := filepath.Join(tempDir, "payload.json")
		if err := os.WriteFile(payloadPath, []byte(`{"id":"a","name":"beta"}`), 0o600); err != nil {
			t.Fatalf("failed to write payload file: %v", err)
		}

		output, err := executeForTest(testDeps(), "", "ad-hoc", "put", "/items/a", "--file", payloadPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"PUT\"") {
			t.Fatalf("expected PUT method output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"beta\"") {
			t.Fatalf("expected file payload to be forwarded, got %q", output)
		}
	})

	t.Run("post_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "post", "/items", "--payload", `{"id":"a","name":"gamma"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"POST\"") {
			t.Fatalf("expected POST method output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"gamma\"") {
			t.Fatalf("expected payload flag body to be forwarded, got %q", output)
		}
	})

	t.Run("put_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "put", "/items/a", "--payload", `{"id":"a","name":"delta"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"PUT\"") {
			t.Fatalf("expected PUT method output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"delta\"") {
			t.Fatalf("expected payload flag body to be forwarded, got %q", output)
		}
	})

	t.Run("payload_conflicts_with_file", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		payloadPath := filepath.Join(tempDir, "payload.json")
		if err := os.WriteFile(payloadPath, []byte(`{"id":"a","name":"beta"}`), 0o600); err != nil {
			t.Fatalf("failed to write payload file: %v", err)
		}

		_, err := executeForTest(
			testDeps(),
			"",
			"ad-hoc",
			"post",
			"/items",
			"--payload",
			`{"id":"a","name":"gamma"}`,
			"--file",
			payloadPath,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("payload_conflicts_with_stdin", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(
			testDeps(),
			`{"id":"a","name":"from-stdin"}`,
			"ad-hoc",
			"post",
			"/items",
			"--payload",
			`{"id":"a","name":"from-flag"}`,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("delete_requires_force", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "ad-hoc", "delete", "/items/a")
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "--force") {
			t.Fatalf("expected --force validation message, got %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "are you sure") {
			t.Fatalf("expected confirmation-style validation message, got %v", err)
		}
	})

	t.Run("delete_with_force", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "delete", "/items/a", "--force")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"DELETE\"") {
			t.Fatalf("expected DELETE method output, got %q", output)
		}
		if !strings.Contains(output, "\"path\": \"/items/a\"") {
			t.Fatalf("expected delete path output, got %q", output)
		}
	})

	t.Run("path_mismatch_fails_validation", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "ad-hoc", "delete", "/a", "--path", "/b", "--force")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceSaveInputModes(t *testing.T) {
	t.Parallel()

	t.Run("without_input_fetches_remote_and_saves_locally", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			"",
			"resource",
			"save",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reconciler.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(reconciler.saveCalls))
		}
		if reconciler.saveCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected save path /customers/acme, got %q", reconciler.saveCalls[0].logicalPath)
		}
		saved, ok := reconciler.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", reconciler.saveCalls[0].value)
		}
		if saved["source"] != "remote" {
			t.Fatalf("expected saved payload to come from remote source, got %#v", saved)
		}
	})

	t.Run("without_input_remote_list_falls_back_to_common_item_identity_attributes", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/clients"] = metadatadomain.ResourceMetadata{
			AliasFromAttribute: "clientId",
		}
		reconciler := &testReconciler{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a-id", "enabled": true},
				map[string]any{"id": "app-b-id", "enabled": false},
			},
		}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			"",
			"resource",
			"save",
			"/admin/realms/master/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reconciler.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(reconciler.saveCalls))
		}
		if reconciler.saveCalls[0].logicalPath != "/admin/realms/master/clients/app-a-id" {
			t.Fatalf("expected first saved path to use id fallback, got %q", reconciler.saveCalls[0].logicalPath)
		}
		if reconciler.saveCalls[1].logicalPath != "/admin/realms/master/clients/app-b-id" {
			t.Fatalf("expected second saved path to use id fallback, got %q", reconciler.saveCalls[1].logicalPath)
		}
	})

	t.Run("default_list_payload_saves_as_items", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`[{"id":"acme","tier":"pro"},{"id":"beta","tier":"free"}]`,
			"resource",
			"save",
			"/customers",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(reconciler.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(reconciler.saveCalls))
		}
		if reconciler.saveCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected first saved path /customers/acme, got %q", reconciler.saveCalls[0].logicalPath)
		}
		if reconciler.saveCalls[1].logicalPath != "/customers/beta" {
			t.Fatalf("expected second saved path /customers/beta, got %q", reconciler.saveCalls[1].logicalPath)
		}
	})

	t.Run("as_one_resource_overrides_list_item_save", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`[{"id":"acme"},{"id":"beta"}]`,
			"resource",
			"save",
			"/customers",
			"--as-one-resource",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(reconciler.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(reconciler.saveCalls))
		}
		if reconciler.saveCalls[0].logicalPath != "/customers" {
			t.Fatalf("expected saved path /customers, got %q", reconciler.saveCalls[0].logicalPath)
		}
		if _, ok := reconciler.saveCalls[0].value.([]any); !ok {
			t.Fatalf("expected single saved payload to be list, got %T", reconciler.saveCalls[0].value)
		}
	})

	t.Run("as_items_requires_list_payload", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"id":"acme"}`,
			"resource",
			"save",
			"/customers/acme",
			"--as-items",
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("as_items_and_as_one_resource_conflict", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`[{"id":"acme"}]`,
			"resource",
			"save",
			"/customers",
			"--as-items",
			"--as-one-resource",
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("plaintext_secret_is_blocked_without_ignore", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "--ignore") {
			t.Fatalf("expected --ignore hint, got %q", err.Error())
		}
		if len(reconciler.saveCalls) != 0 {
			t.Fatalf("expected no save calls after safety failure, got %d", len(reconciler.saveCalls))
		}
	})

	t.Run("list_save_blocks_all_items_before_any_write_when_plaintext_secret_is_detected", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`[{"id":"acme","tier":"pro"},{"id":"beta","password":"plain-secret"}]`,
			"resource",
			"save",
			"/customers",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if len(reconciler.saveCalls) != 0 {
			t.Fatalf("expected no partial writes when safety check fails, got %d", len(reconciler.saveCalls))
		}
	})

	t.Run("metadata_secrets_from_attributes_blocks_plaintext", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"credentials.authValue"},
		}
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"credentials":{"authValue":"plain-secret"}}`,
			"resource",
			"save",
			"/customers/acme",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "credentials.authValue") {
			t.Fatalf("expected metadata attribute in error, got %q", err.Error())
		}
		if len(reconciler.saveCalls) != 0 {
			t.Fatalf("expected no save calls after safety failure, got %d", len(reconciler.saveCalls))
		}
	})

	t.Run("metadata_secrets_from_attributes_accepts_placeholders", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"credentials.authValue"},
		}
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"credentials":{"authValue":"{{secret \"authValue\"}}"}}`,
			"resource",
			"save",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reconciler.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(reconciler.saveCalls))
		}
	})

	t.Run("ignore_flag_allows_plaintext_secret", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
			"--ignore",
		)
		if err != nil {
			t.Fatalf("unexpected error with --ignore: %v", err)
		}
		if len(reconciler.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(reconciler.saveCalls))
		}
	})

	t.Run("handle_secrets_masks_payload_updates_metadata_and_stores_secret_values", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:       "id",
			SecretsFromAttributes: []string{"credentials.authValue", "existingSecret"},
		}
		reconciler := &testReconciler{metadataService: metadataService}
		deps := testDepsWith(reconciler, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)

		_, err := executeForTest(
			deps,
			`{"apiToken":"token-abc","credentials":{"authValue":"plain-secret"}}`,
			"resource",
			"save",
			"/customers/acme",
			"--handle-secrets",
		)
		if err != nil {
			t.Fatalf("unexpected error with --handle-secrets: %v", err)
		}
		if len(reconciler.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(reconciler.saveCalls))
		}

		saved, ok := reconciler.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", reconciler.saveCalls[0].value)
		}
		if got := saved["apiToken"]; got != `{{secret "/customers/acme:apiToken"}}` {
			t.Fatalf("expected apiToken placeholder, got %#v", got)
		}
		credentials, ok := saved["credentials"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested credentials payload, got %T", saved["credentials"])
		}
		if got := credentials["authValue"]; got != `{{secret "/customers/acme:credentials.authValue"}}` {
			t.Fatalf("expected authValue placeholder, got %#v", got)
		}

		if secretProvider.values["/customers/acme:apiToken"] != "token-abc" {
			t.Fatalf("expected stored apiToken value, got %#v", secretProvider.values)
		}
		if secretProvider.values["/customers/acme:credentials.authValue"] != "plain-secret" {
			t.Fatalf("expected stored authValue value, got %#v", secretProvider.values)
		}

		updatedMetadata := metadataService.items["/customers/acme"]
		expected := []string{"apiToken", "credentials.authValue", "existingSecret"}
		if !reflect.DeepEqual(updatedMetadata.SecretsFromAttributes, expected) {
			t.Fatalf("expected merged metadata attributes %#v, got %#v", expected, updatedMetadata.SecretsFromAttributes)
		}
	})

	t.Run("handle_secrets_list_payload_uses_path_scoped_secret_keys", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{metadataService: metadataService}
		deps := testDepsWith(reconciler, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)

		_, err := executeForTest(
			deps,
			`[{"id":"acme","password":"alpha-secret"},{"id":"beta","password":"beta-secret"}]`,
			"resource",
			"save",
			"/customers",
			"--handle-secrets",
		)
		if err != nil {
			t.Fatalf("unexpected list save error with --handle-secrets: %v", err)
		}
		if len(reconciler.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(reconciler.saveCalls))
		}

		firstSaved, ok := reconciler.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected first saved payload map, got %T", reconciler.saveCalls[0].value)
		}
		secondSaved, ok := reconciler.saveCalls[1].value.(map[string]any)
		if !ok {
			t.Fatalf("expected second saved payload map, got %T", reconciler.saveCalls[1].value)
		}
		if got := firstSaved["password"]; got != `{{secret "/customers/acme:password"}}` {
			t.Fatalf("expected first path-scoped placeholder, got %#v", got)
		}
		if got := secondSaved["password"]; got != `{{secret "/customers/beta:password"}}` {
			t.Fatalf("expected second path-scoped placeholder, got %#v", got)
		}

		if secretProvider.values["/customers/acme:password"] != "alpha-secret" {
			t.Fatalf("expected /customers/acme password stored, got %#v", secretProvider.values)
		}
		if secretProvider.values["/customers/beta:password"] != "beta-secret" {
			t.Fatalf("expected /customers/beta password stored, got %#v", secretProvider.values)
		}
	})

	t.Run("handle_secrets_requires_secret_provider_when_candidates_exist", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}
		deps := testDepsWith(reconciler, metadataService)
		deps.Secrets = nil

		_, err := executeForTest(
			deps,
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
			"--handle-secrets",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "secret provider is not configured") {
			t.Fatalf("expected missing secret provider error, got %q", err.Error())
		}
	})

	t.Run("handle_secrets_with_subset_fails_on_remaining_candidates_after_handling_requested", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}
		deps := testDepsWith(reconciler, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)

		_, err := executeForTest(
			deps,
			`{"apiToken":"token-abc","password":"pw-123"}`,
			"resource",
			"save",
			"/customers/acme",
			"--handle-secrets=password",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), `attributes [apiToken]`) {
			t.Fatalf("expected warning with only unhandled secret candidate, got %q", err.Error())
		}
		if len(reconciler.saveCalls) != 0 {
			t.Fatalf("expected no save calls when unhandled secrets remain, got %d", len(reconciler.saveCalls))
		}
		if secretProvider.values["/customers/acme:password"] != "pw-123" {
			t.Fatalf("expected requested secret candidate to be stored, got %#v", secretProvider.values)
		}
		if _, found := secretProvider.values["/customers/acme:apiToken"]; found {
			t.Fatalf("expected unhandled candidate to not be stored, got %#v", secretProvider.values)
		}
	})

	t.Run("handle_secrets_with_unknown_candidate_fails_validation", func(t *testing.T) {
		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"password":"pw-123"}`,
			"resource",
			"save",
			"/customers/acme",
			"--handle-secrets=apiToken",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), `requested --handle-secrets attribute "apiToken" was not detected`) {
			t.Fatalf("expected unknown requested candidate error, got %q", err.Error())
		}
	})

	t.Run("remote_list_handle_secrets_subset_updates_wildcard_metadata_then_fails_on_unhandled", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/clients"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a", "secret": "sec-a", "apiToken": "tok-a"},
				map[string]any{"id": "app-b", "secret": "sec-b", "apiToken": "tok-b"},
			},
		}
		deps := testDepsWith(reconciler, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/master/clients",
			"--handle-secrets=secret",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), `attributes [apiToken]`) {
			t.Fatalf("expected warning with only unhandled secret candidate, got %q", err.Error())
		}
		if len(reconciler.saveCalls) != 0 {
			t.Fatalf("expected no save calls when unhandled secrets remain, got %d", len(reconciler.saveCalls))
		}

		wildcardMetadata := metadataService.items["/admin/realms/*/clients"]
		if !reflect.DeepEqual(wildcardMetadata.SecretsFromAttributes, []string{"secret"}) {
			t.Fatalf("expected wildcard metadata secretsFromAttributes to include secret, got %#v", wildcardMetadata.SecretsFromAttributes)
		}
		if secretProvider.values["/admin/realms/master/clients/app-a:secret"] != "sec-a" {
			t.Fatalf("expected app-a secret to be stored, got %#v", secretProvider.values)
		}
		if _, found := secretProvider.values["/admin/realms/master/clients/app-a:apiToken"]; found {
			t.Fatalf("expected unhandled apiToken to not be stored, got %#v", secretProvider.values)
		}
	})

	t.Run("remote_list_handle_secrets_subset_with_ignore_saves_items", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/clients"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		reconciler := &testReconciler{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a", "secret": "sec-a", "apiToken": "tok-a"},
				map[string]any{"id": "app-b", "secret": "sec-b", "apiToken": "tok-b"},
			},
		}
		deps := testDepsWith(reconciler, metadataService)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/master/clients",
			"--handle-secrets=secret",
			"--ignore",
		)
		if err != nil {
			t.Fatalf("unexpected error with --handle-secrets and --ignore: %v", err)
		}
		if len(reconciler.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(reconciler.saveCalls))
		}

		firstPayload, ok := reconciler.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected first saved payload map, got %T", reconciler.saveCalls[0].value)
		}
		if got := firstPayload["secret"]; got != `{{secret "/admin/realms/master/clients/app-a:secret"}}` {
			t.Fatalf("expected first secret placeholder, got %#v", got)
		}
		if got := firstPayload["apiToken"]; got != "tok-a" {
			t.Fatalf("expected unhandled apiToken to remain plaintext, got %#v", got)
		}

		wildcardMetadata := metadataService.items["/admin/realms/*/clients"]
		if !reflect.DeepEqual(wildcardMetadata.SecretsFromAttributes, []string{"secret"}) {
			t.Fatalf("expected wildcard metadata secretsFromAttributes to include secret, got %#v", wildcardMetadata.SecretsFromAttributes)
		}
	})
}

func TestResourceDefaultOutputUsesContextResourceFormat(t *testing.T) {
	t.Parallel()

	t.Run("json_context_defaults_to_json_output", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"path\"") {
			t.Fatalf("expected json output, got %q", output)
		}
	})

	t.Run("yaml_context_defaults_to_yaml_output", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "-c", "yaml", "resource", "get", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "path: /customers/acme") {
			t.Fatalf("expected yaml output, got %q", output)
		}
	})
}

func TestResourceDeleteRequiresForce(t *testing.T) {
	t.Parallel()

	_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme")
	assertTypedCategory(t, err, faults.ValidationError)
	if !strings.Contains(err.Error(), "flag --force is required") {
		t.Fatalf("expected --force validation message, got %v", err)
	}

	_, err = executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--force")
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}
}

func TestResourceDeleteSourceFlags(t *testing.T) {
	t.Parallel()

	t.Run("default_deletes_from_remote_server", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		reconciler := deps.Orchestrator.(*testReconciler)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(reconciler.deleteCalls) != 1 {
			t.Fatalf("expected 1 remote delete call, got %d", len(reconciler.deleteCalls))
		}
		if len(repositoryService.deleteCalls) != 0 {
			t.Fatalf("expected 0 repository delete calls, got %d", len(repositoryService.deleteCalls))
		}
	})

	t.Run("repository_flag_deletes_only_from_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		reconciler := deps.Orchestrator.(*testReconciler)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force", "--repository")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(reconciler.deleteCalls) != 0 {
			t.Fatalf("expected 0 remote delete calls, got %d", len(reconciler.deleteCalls))
		}
		if len(repositoryService.deleteCalls) != 1 {
			t.Fatalf("expected 1 repository delete call, got %d", len(repositoryService.deleteCalls))
		}
	})

	t.Run("both_flag_deletes_from_remote_and_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		reconciler := deps.Orchestrator.(*testReconciler)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force", "--both")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(reconciler.deleteCalls) != 1 {
			t.Fatalf("expected 1 remote delete call, got %d", len(reconciler.deleteCalls))
		}
		if len(repositoryService.deleteCalls) != 1 {
			t.Fatalf("expected 1 repository delete call, got %d", len(repositoryService.deleteCalls))
		}
	})

	t.Run("source_flags_are_mutually_exclusive", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--force", "--repository", "--both")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestMetadataPathCommands(t *testing.T) {
	t.Parallel()

	t.Run("resolve_with_path_returns_metadata", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "metadata", "resolve", "--path", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected resolve error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"id\"") {
			t.Fatalf("expected resolved metadata output, got %q", output)
		}
	})

	t.Run("resolve_missing_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "metadata", "resolve")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("render_positional_path", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "metadata", "render", "/customers/acme", "get")
		if err != nil {
			t.Fatalf("unexpected render error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/api/customers/acme\"") {
			t.Fatalf("expected rendered operation spec output, got %q", output)
		}
	})

	t.Run("render_flag_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "metadata", "render", "--path", "/customers/acme", "get")
		if err != nil {
			t.Fatalf("unexpected render error with --path: %v", err)
		}
	})

	t.Run("render_mismatch_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "metadata", "render", "/customers/a", "--path", "/customers/b", "get")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("render_missing_operation", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "metadata", "render", "/customers/acme")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestSecretCommands(t *testing.T) {
	t.Parallel()

	t.Run("store_get_and_delete_roundtrip", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		if _, err := executeForTest(deps, "", "secret", "store", "apiToken", "token-123"); err != nil {
			t.Fatalf("store returned error: %v", err)
		}

		output, err := executeForTest(deps, "", "secret", "get", "apiToken")
		if err != nil {
			t.Fatalf("get returned error: %v", err)
		}
		if !strings.Contains(output, "token-123") {
			t.Fatalf("expected stored value in output, got %q", output)
		}

		if _, err := executeForTest(deps, "", "secret", "delete", "apiToken"); err != nil {
			t.Fatalf("delete returned error: %v", err)
		}

		_, err = executeForTest(deps, "", "secret", "get", "apiToken")
		assertTypedCategory(t, err, faults.NotFoundError)
	})

	t.Run("mask_and_resolve_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		payload := `{"apiToken":"token-abc","name":"acme"}`
		masked, err := executeForTest(deps, payload, "secret", "mask")
		if err != nil {
			t.Fatalf("mask returned error: %v", err)
		}
		if !strings.Contains(masked, `{{secret \"apiToken\"}}`) {
			t.Fatalf("expected masked placeholder, got %q", masked)
		}

		resolved, err := executeForTest(deps, masked, "secret", "resolve")
		if err != nil {
			t.Fatalf("resolve returned error: %v", err)
		}
		if !strings.Contains(resolved, "token-abc") {
			t.Fatalf("expected resolved secret value, got %q", resolved)
		}
	})

	t.Run("detect_payload_candidates", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		payload := `{"apiToken":"token-abc","password":"pw-123","nested":{"apiToken":"{{secret \"apiToken\"}}"}}`
		detected, err := executeForTest(deps, payload, "secret", "detect")
		if err != nil {
			t.Fatalf("detect returned error: %v", err)
		}
		if !strings.Contains(detected, `"apiToken"`) || !strings.Contains(detected, `"password"`) {
			t.Fatalf("expected detected candidates in output, got %q", detected)
		}
	})

	t.Run("detect_without_input_scans_whole_repository", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
				"/customers/beta": map[string]any{"apiToken": "token-abc"},
			},
		}

		output, err := executeForTest(testDepsWith(reconciler, metadataService), "", "secret", "detect")
		if err != nil {
			t.Fatalf("detect without input returned error: %v", err)
		}
		if len(reconciler.listLocalCalls) != 1 || reconciler.listLocalCalls[0] != "/" {
			t.Fatalf("expected repo-wide scan with path \"/\", got %#v", reconciler.listLocalCalls)
		}
		if !strings.Contains(output, "\"LogicalPath\": \"/customers/acme\"") ||
			!strings.Contains(output, "\"LogicalPath\": \"/customers/beta\"") {
			t.Fatalf("expected repo scan output to include both resources, got %q", output)
		}
		if !strings.Contains(output, "\"Attributes\": [\n      \"password\"\n    ]") ||
			!strings.Contains(output, "\"Attributes\": [\n      \"apiToken\"\n    ]") {
			t.Fatalf("expected detected attributes per resource, got %q", output)
		}
	})

	t.Run("detect_without_input_scopes_to_path", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
			},
		}

		output, err := executeForTest(testDepsWith(reconciler, metadataService), "", "secret", "detect", "/customers")
		if err != nil {
			t.Fatalf("detect path scope without input returned error: %v", err)
		}
		if len(reconciler.listLocalCalls) != 1 || reconciler.listLocalCalls[0] != "/customers" {
			t.Fatalf("expected scoped scan with path \"/customers\", got %#v", reconciler.listLocalCalls)
		}
		if !strings.Contains(output, "\"LogicalPath\": \"/customers/acme\"") {
			t.Fatalf("expected scoped detect output, got %q", output)
		}
	})

	t.Run("detect_fix_updates_metadata_for_target_path", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"apiToken":"token-abc","password":"pw-123","name":"acme"}`,
			"secret",
			"detect",
			"/customers/acme",
			"--fix",
		)
		if err != nil {
			t.Fatalf("detect --fix returned error: %v", err)
		}

		updated := metadataService.items["/customers/acme"]
		expected := []string{"apiToken", "password"}
		if !reflect.DeepEqual(updated.SecretsFromAttributes, expected) {
			t.Fatalf("expected secretsFromAttributes %#v, got %#v", expected, updated.SecretsFromAttributes)
		}
		if updated.IDFromAttribute != "id" {
			t.Fatalf("expected existing idFromAttribute to be preserved, got %q", updated.IDFromAttribute)
		}
	})

	t.Run("detect_fix_updates_metadata_for_target_path_flag", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"password":"pw-123","name":"acme"}`,
			"secret",
			"detect",
			"--fix",
			"--path",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("detect --fix --path returned error: %v", err)
		}

		updated := metadataService.items["/customers/acme"]
		expected := []string{"password"}
		if !reflect.DeepEqual(updated.SecretsFromAttributes, expected) {
			t.Fatalf("expected secretsFromAttributes %#v, got %#v", expected, updated.SecretsFromAttributes)
		}
	})

	t.Run("detect_without_input_fix_updates_metadata_for_detected_paths", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
				"/customers/beta": map[string]any{"apiToken": "token-abc"},
			},
		}

		_, err := executeForTest(testDepsWith(reconciler, metadataService), "", "secret", "detect", "--fix")
		if err != nil {
			t.Fatalf("detect --fix without input returned error: %v", err)
		}

		if !reflect.DeepEqual(metadataService.items["/customers/acme"].SecretsFromAttributes, []string{"password"}) {
			t.Fatalf("expected /customers/acme metadata update, got %#v", metadataService.items["/customers/acme"].SecretsFromAttributes)
		}
		if !reflect.DeepEqual(metadataService.items["/customers/beta"].SecretsFromAttributes, []string{"apiToken"}) {
			t.Fatalf("expected /customers/beta metadata update, got %#v", metadataService.items["/customers/beta"].SecretsFromAttributes)
		}
	})

	t.Run("detect_fix_requires_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), `{"password":"pw-123"}`, "secret", "detect", "--fix")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("detect_path_without_fix_fails_when_using_payload_input", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), `{"password":"pw-123"}`, "secret", "detect", "/customers/acme")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("detect_path_flag_without_fix_fails_when_using_payload_input", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), `{"password":"pw-123"}`, "secret", "detect", "--path", "/customers/acme")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("detect_fix_with_secret_attribute_filters_applied_attribute", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:       "id",
			SecretsFromAttributes: []string{"password"},
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationGet): {Path: "/api/customers/acme"},
			},
		}
		reconciler := &testReconciler{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"apiToken":"token-abc","password":"pw-123"}`,
			"secret",
			"detect",
			"/customers/acme",
			"--fix",
			"--secret-attribute",
			"apiToken",
		)
		if err != nil {
			t.Fatalf("detect --fix --secret-attribute returned error: %v", err)
		}
		if !strings.Contains(output, `"apiToken"`) || strings.Contains(output, `"password"`) {
			t.Fatalf("expected filtered output with only apiToken, got %q", output)
		}

		updated := metadataService.items["/customers/acme"]
		expected := []string{"apiToken", "password"}
		if !reflect.DeepEqual(updated.SecretsFromAttributes, expected) {
			t.Fatalf("expected merged secretsFromAttributes %#v, got %#v", expected, updated.SecretsFromAttributes)
		}
	})

	t.Run("detect_fix_with_missing_secret_attribute_fails", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			`{"password":"pw-123"}`,
			"secret",
			"detect",
			"/customers/acme",
			"--fix",
			"--secret-attribute",
			"apiToken",
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("detect_without_input_secret_attribute_filter", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
				"/customers/beta": map[string]any{"apiToken": "token-abc"},
			},
		}

		output, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			"",
			"secret",
			"detect",
			"--secret-attribute",
			"password",
		)
		if err != nil {
			t.Fatalf("detect repo scan --secret-attribute returned error: %v", err)
		}
		if !strings.Contains(output, "\"LogicalPath\": \"/customers/acme\"") || strings.Contains(output, "/customers/beta") {
			t.Fatalf("expected only matching resources in filtered output, got %q", output)
		}
	})

	t.Run("detect_without_input_missing_secret_attribute_fails", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		reconciler := &testReconciler{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
			},
		}

		_, err := executeForTest(
			testDepsWith(reconciler, metadataService),
			"",
			"secret",
			"detect",
			"--secret-attribute",
			"apiToken",
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestRepoStatusOutput(t *testing.T) {
	t.Parallel()

	textOutput, err := executeForTest(testDeps(), "", "repo", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(textOutput, "state=no_remote") {
		t.Fatalf("expected text repo status output, got %q", textOutput)
	}

	jsonOutput, err := executeForTest(testDeps(), "", "-o", "json", "repo", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(jsonOutput, "\"state\": \"no_remote\"") {
		t.Fatalf("expected structured json status output, got %q", jsonOutput)
	}
}

func TestResourceListRecursiveFlag(t *testing.T) {
	t.Parallel()

	directOutput, err := executeForTest(testDeps(), "", "resource", "list", "/customers")
	if err != nil {
		t.Fatalf("unexpected direct list error: %v", err)
	}
	if !strings.Contains(directOutput, "\"path\": \"/customers\"") {
		t.Fatalf("expected direct list payload output, got %q", directOutput)
	}
	if strings.Contains(directOutput, "\"LogicalPath\"") {
		t.Fatalf("expected payload-only direct list output, got %q", directOutput)
	}

	recursiveOutput, err := executeForTest(testDeps(), "", "resource", "list", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive list error: %v", err)
	}
	if !strings.Contains(recursiveOutput, "\"path\": \"/customers/nested\"") {
		t.Fatalf("expected recursive list payload output, got %q", recursiveOutput)
	}
	if strings.Contains(recursiveOutput, "\"LogicalPath\"") {
		t.Fatalf("expected payload-only recursive list output, got %q", recursiveOutput)
	}
}

func TestResourceListSourceFlags(t *testing.T) {
	t.Parallel()

	t.Run("default_lists_from_remote_server", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(reconciler, reconciler.metadataService), "", "resource", "list", "/")
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if !strings.Contains(output, "\"id\": \"remote-only\"") {
			t.Fatalf("expected remote-server output by default, got %q", output)
		}
		if strings.Contains(output, "\"id\": \"repo-only\"") {
			t.Fatalf("expected repository output to be absent by default, got %q", output)
		}
		if strings.Contains(output, "\"LogicalPath\"") {
			t.Fatalf("expected payload-only list output, got %q", output)
		}
	})

	t.Run("remote_server_flag_lists_from_remote_server", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(reconciler, reconciler.metadataService), "", "resource", "list", "/", "--remote-server")
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if !strings.Contains(output, "\"id\": \"remote-only\"") {
			t.Fatalf("expected remote-server output with --remote-server, got %q", output)
		}
		if strings.Contains(output, "\"id\": \"repo-only\"") {
			t.Fatalf("expected repository output to be absent with --remote-server, got %q", output)
		}
		if strings.Contains(output, "\"LogicalPath\"") {
			t.Fatalf("expected payload-only list output, got %q", output)
		}
	})

	t.Run("repository_flag_lists_from_repository", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(reconciler, reconciler.metadataService), "", "resource", "list", "/", "--repository")
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if !strings.Contains(output, "\"id\": \"repo-only\"") {
			t.Fatalf("expected repository output, got %q", output)
		}
		if strings.Contains(output, "\"id\": \"remote-only\"") {
			t.Fatalf("expected remote output to be absent with --repository, got %q", output)
		}
		if strings.Contains(output, "\"LogicalPath\"") {
			t.Fatalf("expected payload-only list output, got %q", output)
		}
	})

	t.Run("repository_and_remote_server_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "list", "/", "--repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceApplyCollectionPath(t *testing.T) {
	t.Parallel()

	reconciler := &testReconciler{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
			{LogicalPath: "/customers/beta"},
			{LogicalPath: "/customers/nested/gamma"},
		},
	}
	deps := testDepsWith(reconciler, reconciler.metadataService)

	directOutput, err := executeForTest(deps, "", "resource", "apply", "/customers")
	if err != nil {
		t.Fatalf("unexpected direct apply error: %v", err)
	}
	expectedDirectCalls := []string{"/customers/acme", "/customers/beta"}
	if !reflect.DeepEqual(reconciler.applyCalls, expectedDirectCalls) {
		t.Fatalf("expected direct apply calls %#v, got %#v", expectedDirectCalls, reconciler.applyCalls)
	}
	if !strings.Contains(directOutput, "\"LogicalPath\": \"/customers/acme\"") ||
		!strings.Contains(directOutput, "\"LogicalPath\": \"/customers/beta\"") {
		t.Fatalf("expected direct apply output for direct resources, got %q", directOutput)
	}
	if strings.Contains(directOutput, "/customers/nested/gamma") {
		t.Fatalf("expected non-recursive apply to exclude nested resources, got %q", directOutput)
	}

	reconciler.applyCalls = nil
	recursiveOutput, err := executeForTest(deps, "", "resource", "apply", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive apply error: %v", err)
	}
	expectedRecursiveCalls := []string{"/customers/acme", "/customers/beta", "/customers/nested/gamma"}
	if !reflect.DeepEqual(reconciler.applyCalls, expectedRecursiveCalls) {
		t.Fatalf("expected recursive apply calls %#v, got %#v", expectedRecursiveCalls, reconciler.applyCalls)
	}
	if !strings.Contains(recursiveOutput, "\"LogicalPath\": \"/customers/nested/gamma\"") {
		t.Fatalf("expected recursive apply output to include nested resource, got %q", recursiveOutput)
	}
}

func TestResourceCreateRequiresInputAndRunsSingleMutation(t *testing.T) {
	t.Parallel()

	t.Run("create_with_input", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{metadataService: newTestMetadata()}
		deps := testDepsWith(reconciler, reconciler.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "create", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if len(reconciler.createCalls) != 1 {
			t.Fatalf("expected single create call, got %d", len(reconciler.createCalls))
		}
		if reconciler.createCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected create path /customers/acme, got %q", reconciler.createCalls[0].logicalPath)
		}
		if !strings.Contains(output, "/customers/acme") {
			t.Fatalf("expected create output to contain /customers/acme, got %q", output)
		}
	})

	t.Run("create_without_input_fails", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{metadataService: newTestMetadata()}
		deps := testDepsWith(reconciler, reconciler.metadataService)

		_, err := executeForTest(deps, "", "resource", "create", "/customers/acme")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("create_with_payload_flag", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{metadataService: newTestMetadata()}
		deps := testDepsWith(reconciler, reconciler.metadataService)

		output, err := executeForTest(deps, "", "resource", "create", "/customers/acme", "--payload", `{"id":"acme","tier":"startup"}`)
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if len(reconciler.createCalls) != 1 {
			t.Fatalf("expected single create call, got %d", len(reconciler.createCalls))
		}
		if !strings.Contains(output, "/customers/acme") {
			t.Fatalf("expected create output to contain /customers/acme, got %q", output)
		}
	})

	t.Run("create_payload_conflicts_with_file", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{metadataService: newTestMetadata()}
		deps := testDepsWith(reconciler, reconciler.metadataService)
		tempDir := t.TempDir()
		payloadPath := filepath.Join(tempDir, "payload.json")
		if err := os.WriteFile(payloadPath, []byte(`{"id":"acme","tier":"pro"}`), 0o600); err != nil {
			t.Fatalf("failed to write payload file: %v", err)
		}

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"create",
			"/customers/acme",
			"--payload",
			`{"id":"acme","tier":"startup"}`,
			"--file",
			payloadPath,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("create_payload_conflicts_with_stdin", func(t *testing.T) {
		t.Parallel()

		reconciler := &testReconciler{metadataService: newTestMetadata()}
		deps := testDepsWith(reconciler, reconciler.metadataService)

		_, err := executeForTest(
			deps,
			`{"id":"acme","tier":"from-stdin"}`,
			"resource",
			"create",
			"/customers/acme",
			"--payload",
			`{"id":"acme","tier":"from-flag"}`,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceUpdateUsesRepositoryPayloads(t *testing.T) {
	t.Parallel()

	reconciler := &testReconciler{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
			{LogicalPath: "/customers/nested/gamma"},
		},
		getLocalValues: map[string]resource.Value{
			"/customers/acme":         map[string]any{"id": "acme", "tier": "pro"},
			"/customers/nested/gamma": map[string]any{"id": "gamma", "tier": "free"},
		},
	}
	deps := testDepsWith(reconciler, reconciler.metadataService)

	output, err := executeForTest(deps, "", "resource", "update", "/customers")
	if err != nil {
		t.Fatalf("unexpected update collection error: %v", err)
	}
	if len(reconciler.updateCalls) != 1 {
		t.Fatalf("expected 1 update call for non-recursive update, got %d", len(reconciler.updateCalls))
	}
	if reconciler.updateCalls[0].logicalPath != "/customers/acme" {
		t.Fatalf("expected non-recursive update path /customers/acme, got %q", reconciler.updateCalls[0].logicalPath)
	}
	if len(reconciler.getLocalCalls) != 1 || reconciler.getLocalCalls[0] != "/customers/acme" {
		t.Fatalf("expected non-recursive update to read only /customers/acme, got %#v", reconciler.getLocalCalls)
	}
	if !reflect.DeepEqual(reconciler.updateCalls[0].value, reconciler.getLocalValues["/customers/acme"]) {
		t.Fatalf("expected update payload to come from local resource for /customers/acme")
	}
	if !strings.Contains(output, "\"LogicalPath\": \"/customers/acme\"") {
		t.Fatalf("expected update output to include /customers/acme, got %q", output)
	}
	if strings.Contains(output, "/customers/nested/gamma") {
		t.Fatalf("expected non-recursive update output to exclude nested resources, got %q", output)
	}

	reconciler.updateCalls = nil
	reconciler.getLocalCalls = nil
	recursiveOutput, err := executeForTest(deps, "", "resource", "update", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive update error: %v", err)
	}
	if len(reconciler.updateCalls) != 2 {
		t.Fatalf("expected 2 update calls for recursive update, got %d", len(reconciler.updateCalls))
	}
	if !strings.Contains(recursiveOutput, "/customers/nested/gamma") {
		t.Fatalf("expected recursive update output to include nested resources, got %q", recursiveOutput)
	}
}

func TestResourceCollectionMutationsFallbackToSingleResourceLookupWhenListIsEmpty(t *testing.T) {
	t.Parallel()

	const idPath = "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c"

	reconciler := &testReconciler{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/admin/realms/master/clients/account"},
		},
		getLocalValues: map[string]resource.Value{
			idPath: map[string]any{"id": "f88c68f3-3253-49f9-94a9-fe7553d33b5c", "clientId": "account"},
		},
	}
	deps := testDepsWith(reconciler, reconciler.metadataService)

	_, err := executeForTest(deps, "", "resource", "apply", idPath)
	if err != nil {
		t.Fatalf("unexpected apply fallback error: %v", err)
	}
	if len(reconciler.applyCalls) != 1 || reconciler.applyCalls[0] != idPath {
		t.Fatalf("expected apply to execute fallback single target %q, got %#v", idPath, reconciler.applyCalls)
	}

	reconciler.updateCalls = nil
	reconciler.getLocalCalls = nil
	_, err = executeForTest(deps, "", "resource", "update", idPath)
	if err != nil {
		t.Fatalf("unexpected update fallback error: %v", err)
	}
	if len(reconciler.updateCalls) != 1 || reconciler.updateCalls[0].logicalPath != idPath {
		t.Fatalf("expected update to execute fallback single target %q, got %#v", idPath, reconciler.updateCalls)
	}
	if len(reconciler.getLocalCalls) == 0 || reconciler.getLocalCalls[0] != idPath {
		t.Fatalf("expected getLocal fallback lookup for %q, got %#v", idPath, reconciler.getLocalCalls)
	}
}

func TestResourceCollectionMutationsFailWhenNoLocalResourcesMatch(t *testing.T) {
	t.Parallel()

	reconciler := &testReconciler{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}
	deps := testDepsWith(reconciler, reconciler.metadataService)

	_, err := executeForTest(deps, "", "resource", "apply", "/orders")
	assertTypedCategory(t, err, faults.NotFoundError)

	_, err = executeForTest(deps, "", "resource", "create", "/orders")
	assertTypedCategory(t, err, faults.ValidationError)

	_, err = executeForTest(deps, "", "resource", "update", "/orders")
	assertTypedCategory(t, err, faults.NotFoundError)
}

func TestCompletionBashGeneratesScript(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "completion", "bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "declarest") {
		t.Fatalf("expected completion script output, got %q", output)
	}
}

func TestPathCompletionMergesLocalRemoteAndOpenAPI(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	reconciler := deps.Orchestrator.(*testReconciler)
	reconciler.localList = []resource.Resource{
		{LogicalPath: "/customers/local"},
	}
	reconciler.remoteList = []resource.Resource{
		{LogicalPath: "/customers/remote"},
	}
	reconciler.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/customers/{id}": map[string]any{},
			"/health":         map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/customers")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/customers/local") {
		t.Fatalf("expected local path completion, got %q", output)
	}
	if !strings.Contains(output, "/customers/remote") {
		t.Fatalf("expected remote path completion, got %q", output)
	}
	if !strings.Contains(output, "/customers/{id}") {
		t.Fatalf("expected OpenAPI path completion, got %q", output)
	}
	if !strings.Contains(output, ":4") {
		t.Fatalf("expected no-file completion directive, got %q", output)
	}
}

func TestContextFlagCompletionShowsContextNames(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "get", "--context", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "dev") || !strings.Contains(output, "prod") {
		t.Fatalf("expected context names in completion output, got %q", output)
	}
}

func TestOutputFlagCompletionShowsSupportedValues(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "get", "--output", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	expected := []string{"auto", "text", "json", "yaml"}
	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Fatalf("expected %q output completion value, got %q", value, output)
		}
	}
}

func TestResourceListCompletionShowsSourceFlags(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "resource", "list", "--")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "--repository") || !strings.Contains(output, "--remote-server") {
		t.Fatalf("expected source flag completion values, got %q", output)
	}
}

func TestMetadataRenderCompletionSuggestsOperations(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "metadata", "render", "/customers", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	expected := []string{"get", "create", "update", "delete", "list", "compare"}
	for _, value := range expected {
		if !strings.Contains(output, value) {
			t.Fatalf("expected operation completion value %q, got %q", value, output)
		}
	}

	withPathFlagOutput, err := executeForTest(testDeps(), "", "__complete", "metadata", "render", "--path", "/customers", "")
	if err != nil {
		t.Fatalf("unexpected completion error with --path: %v", err)
	}
	for _, value := range expected {
		if !strings.Contains(withPathFlagOutput, value) {
			t.Fatalf("expected operation completion value %q with --path, got %q", value, withPathFlagOutput)
		}
	}
}

func TestCommandWithoutRequiredSubcommandShowsHelp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		args            []string
		expectedSnippet string
	}{
		{name: "ad-hoc", args: []string{"ad-hoc"}, expectedSnippet: "Execute ad-hoc HTTP requests against managed server API"},
		{name: "config", args: []string{"config"}, expectedSnippet: "Manage contexts"},
		{name: "metadata", args: []string{"metadata"}, expectedSnippet: "Manage metadata"},
		{name: "repo", args: []string{"repo"}, expectedSnippet: "Manage local repository state"},
		{name: "resource", args: []string{"resource"}, expectedSnippet: "Manage resources"},
		{name: "secret", args: []string{"secret"}, expectedSnippet: "Manage secrets"},
		{name: "completion", args: []string{"completion"}, expectedSnippet: "Generate shell completion scripts"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeForTest(testDeps(), "", testCase.args...)
			if err != nil {
				t.Fatalf("expected help output for missing subcommand, got error: %v", err)
			}
			if !strings.Contains(output, testCase.expectedSnippet) {
				t.Fatalf("expected help output to contain %q, got %q", testCase.expectedSnippet, output)
			}
		})
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	t.Parallel()

	_, err := executeForTest(testDeps(), "", "unknown-command")
	if err == nil {
		t.Fatal("expected unknown command to return error")
	}
}

func TestHelpSubcommandEnabled(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "help")
	if err != nil {
		t.Fatalf("expected help subcommand to work: %v", err)
	}
	if !strings.Contains(output, "Manage declarative resources") {
		t.Fatalf("expected root help output, got %q", output)
	}
	if !strings.Contains(output, "help        Help about any command") {
		t.Fatalf("expected canonical help entry in root help output, got %q", output)
	}

	resourceHelpOutput, err := executeForTest(testDeps(), "", "help", "resource")
	if err != nil {
		t.Fatalf("expected nested help to work: %v", err)
	}
	if !strings.Contains(resourceHelpOutput, "Manage resources") {
		t.Fatalf("expected resource help output from help subcommand, got %q", resourceHelpOutput)
	}

	output, err = executeForTest(testDeps(), "", "resource", "--help")
	if err != nil {
		t.Fatalf("expected --help flag to work: %v", err)
	}
	if !strings.Contains(output, "Read a resource") {
		t.Fatalf("expected command help output, got %q", output)
	}
}

func TestResourceSaveHelpIncludesHandleSecretsFlag(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "resource", "save", "--help")
	if err != nil {
		t.Fatalf("expected resource save help output, got error: %v", err)
	}
	if !strings.Contains(output, "--handle-secrets") {
		t.Fatalf("expected --handle-secrets in resource save help output, got %q", output)
	}
}

func TestRootCompletionShowsCanonicalHelpCommand(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "help\tHelp about any command") {
		t.Fatalf("expected help completion entry, got %q", output)
	}
	if strings.Contains(output, "__help") {
		t.Fatalf("expected internal help alias to be absent from completion output, got %q", output)
	}
}

func TestHelpFlagAppearsInGlobalFlagsForAllCommands(t *testing.T) {
	t.Parallel()

	command := NewRootCommand(testDeps())
	paths := append([][]string{{}}, registeredPaths(command, nil)...)

	for _, path := range paths {
		pathCopy := append([]string{}, path...)
		testName := joinPath(pathCopy)
		if testName == "root" {
			testName = "declarest"
		}

		t.Run(testName, func(t *testing.T) {
			args := append(pathCopy, "--help")
			output, err := executeForTest(testDeps(), "", args...)
			if err != nil {
				t.Fatalf("expected help output, got error: %v", err)
			}

			globalFlags := extractHelpSection(output, "Global Flags:")
			if !strings.Contains(globalFlags, "--help") {
				t.Fatalf("expected --help in Global Flags section, got %q", output)
			}

			localFlags := extractHelpSection(output, "Flags:")
			if strings.Contains(localFlags, "--help") {
				t.Fatalf("expected --help to be absent from local Flags section, got %q", output)
			}
		})
	}
}

func TestHelpOutputDoesNotContainExcessiveBlankLines(t *testing.T) {
	t.Parallel()

	command := NewRootCommand(testDeps())
	paths := append([][]string{{}}, registeredPaths(command, nil)...)

	for _, path := range paths {
		pathCopy := append([]string{}, path...)
		testName := joinPath(pathCopy)
		if testName == "root" {
			testName = "declarest"
		}

		t.Run(testName, func(t *testing.T) {
			args := append(pathCopy, "--help")
			output, err := executeForTest(testDeps(), "", args...)
			if err != nil {
				t.Fatalf("expected help output, got error: %v", err)
			}

			if got := trailingBlankLineCount(output); got != 0 {
				t.Fatalf("expected no trailing blank lines in help output, got %d for %q", got, joinPath(pathCopy))
			}
		})
	}
}

func executeForTest(deps Dependencies, stdin string, args ...string) (string, error) {
	output, _, err := executeForTestWithStreams(deps, stdin, args...)
	return output, err
}

func executeForTestWithStreams(deps Dependencies, stdin string, args ...string) (string, string, error) {
	command := NewRootCommand(deps)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetIn(strings.NewReader(stdin))
	command.SetArgs(args)

	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func registeredPaths(command *cobra.Command, prefix []string) [][]string {
	paths := make([][]string, 0)
	for _, child := range command.Commands() {
		name := child.Name()
		if name == "help" || len(name) > 1 && name[:2] == "__" {
			continue
		}
		current := append(append([]string{}, prefix...), name)
		paths = append(paths, current)
		paths = append(paths, registeredPaths(child, current)...)
	}
	return paths
}

func joinPath(path []string) string {
	if len(path) == 0 {
		return "root"
	}
	joined := path[0]
	for i := 1; i < len(path); i++ {
		joined += " " + path[i]
	}
	return joined
}

func extractHelpSection(output string, heading string) string {
	lines := strings.Split(output, "\n")
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return ""
	}

	section := make([]string, 0)
	for index := start; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(section) > 0 {
				break
			}
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":") {
			break
		}

		section = append(section, line)
	}

	return strings.Join(section, "\n")
}

func trailingBlankLineCount(value string) int {
	lines := strings.Split(value, "\n")
	emptySuffix := 0
	for index := len(lines) - 1; index >= 0; index-- {
		if lines[index] != "" {
			break
		}
		emptySuffix++
	}
	if emptySuffix == 0 {
		return 0
	}
	// Account for the expected terminal newline in help output.
	if emptySuffix == 1 {
		return 0
	}
	return emptySuffix - 1
}

func testDeps() Dependencies {
	metadataService := newTestMetadata()
	return testDepsWith(
		&testReconciler{
			metadataService: metadataService,
		},
		metadataService,
	)
}

func testDepsWith(reconciler *testReconciler, metadataService *testMetadata) Dependencies {
	secretProvider := newTestSecretProvider()
	repositoryService := &testRepository{}

	return Dependencies{
		Orchestrator: reconciler,
		Contexts:     &testContextService{},
		Repository:   repositoryService,
		Metadata:     metadataService,
		Secrets:      secretProvider,
	}
}

type testContextService struct{}

func (s *testContextService) Create(context.Context, config.Context) error { return nil }
func (s *testContextService) Update(context.Context, config.Context) error { return nil }
func (s *testContextService) Delete(context.Context, string) error         { return nil }
func (s *testContextService) Rename(context.Context, string, string) error { return nil }
func (s *testContextService) List(context.Context) ([]config.Context, error) {
	return []config.Context{{Name: "dev"}, {Name: "prod"}}, nil
}
func (s *testContextService) SetCurrent(context.Context, string) error { return nil }
func (s *testContextService) GetCurrent(context.Context) (config.Context, error) {
	return config.Context{Name: "dev"}, nil
}
func (s *testContextService) ResolveContext(_ context.Context, selection config.ContextSelection) (config.Context, error) {
	name := selection.Name
	if name == "" {
		name = "dev"
	}
	format := config.ResourceFormatJSON
	if name == "yaml" {
		format = config.ResourceFormatYAML
	}

	return config.Context{
		Name: name,
		Repository: config.Repository{
			ResourceFormat: format,
			Filesystem:     &config.FilesystemRepository{BaseDir: "/tmp/repo"},
		},
	}, nil
}
func (s *testContextService) Validate(context.Context, config.Context) error { return nil }

type testReconciler struct {
	metadataService *testMetadata
	saveCalls       []savedResource
	deleteCalls     []deleteCall
	saveErr         error
	getRemoteValue  resource.Value
	getRemoteErr    error
	getRemoteCalls  []string
	adHocErr        error
	getLocalCalls   []string
	listLocalCalls  []string
	applyCalls      []string
	createCalls     []savedResource
	updateCalls     []savedResource
	getLocalValues  map[string]resource.Value
	localList       []resource.Resource
	remoteList      []resource.Resource
	openAPISpec     resource.Value
}

type savedResource struct {
	logicalPath string
	value       resource.Value
}

type deleteCall struct {
	logicalPath string
	recursive   bool
}

func (r *testReconciler) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath, "source": "get"}, nil
}
func (r *testReconciler) GetLocal(_ context.Context, logicalPath string) (resource.Value, error) {
	r.getLocalCalls = append(r.getLocalCalls, logicalPath)
	if r.getLocalValues != nil {
		if value, ok := r.getLocalValues[logicalPath]; ok {
			return value, nil
		}
	}
	return map[string]any{"path": logicalPath, "source": "local"}, nil
}
func (r *testReconciler) GetRemote(_ context.Context, logicalPath string) (resource.Value, error) {
	r.getRemoteCalls = append(r.getRemoteCalls, logicalPath)
	if r.getRemoteErr != nil {
		return nil, r.getRemoteErr
	}
	if r.getRemoteValue != nil {
		return r.getRemoteValue, nil
	}
	return map[string]any{"path": logicalPath, "source": "remote"}, nil
}
func (r *testReconciler) AdHoc(_ context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error) {
	if r.adHocErr != nil {
		return nil, r.adHocErr
	}
	return map[string]any{
		"method": method,
		"path":   endpointPath,
		"body":   body,
	}, nil
}
func (r *testReconciler) GetOpenAPISpec(_ context.Context) (resource.Value, error) {
	return r.openAPISpec, nil
}
func (r *testReconciler) Save(_ context.Context, logicalPath string, value resource.Value) error {
	r.saveCalls = append(r.saveCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return r.saveErr
}
func (r *testReconciler) Apply(_ context.Context, logicalPath string) (resource.Resource, error) {
	r.applyCalls = append(r.applyCalls, logicalPath)
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Create(_ context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	r.createCalls = append(r.createCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Update(_ context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	r.updateCalls = append(r.updateCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Delete(_ context.Context, logicalPath string, policy orchestrator.DeletePolicy) error {
	r.deleteCalls = append(r.deleteCalls, deleteCall{
		logicalPath: logicalPath,
		recursive:   policy.Recursive,
	})
	return nil
}
func (r *testReconciler) ListLocal(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	r.listLocalCalls = append(r.listLocalCalls, logicalPath)
	if len(r.localList) > 0 {
		items := make([]resource.Resource, len(r.localList))
		copy(items, r.localList)
		if !policy.Recursive {
			filtered := make([]resource.Resource, 0, len(items))
			for _, item := range items {
				if isDirectChildPath(logicalPath, item.LogicalPath) {
					filtered = append(filtered, item)
				}
			}
			return filtered, nil
		}
		return items, nil
	}
	if policy.Recursive {
		return []resource.Resource{{
			LogicalPath: logicalPath + "/nested",
			Payload:     map[string]any{"path": logicalPath + "/nested"},
		}}, nil
	}
	return []resource.Resource{{
		LogicalPath: logicalPath,
		Payload:     map[string]any{"path": logicalPath},
	}}, nil
}
func (r *testReconciler) ListRemote(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	if len(r.remoteList) > 0 {
		items := make([]resource.Resource, len(r.remoteList))
		copy(items, r.remoteList)
		if !policy.Recursive {
			filtered := make([]resource.Resource, 0, len(items))
			for _, item := range items {
				if isDirectChildPath(logicalPath, item.LogicalPath) {
					filtered = append(filtered, item)
				}
			}
			return filtered, nil
		}
		return items, nil
	}
	if policy.Recursive {
		return []resource.Resource{{
			LogicalPath: logicalPath + "/nested",
			Payload:     map[string]any{"path": logicalPath + "/nested"},
		}}, nil
	}
	return []resource.Resource{{
		LogicalPath: logicalPath,
		Payload:     map[string]any{"path": logicalPath},
	}}, nil
}
func (r *testReconciler) Explain(_ context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	return []resource.DiffEntry{{Path: logicalPath, Operation: "noop"}}, nil
}
func (r *testReconciler) Diff(_ context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	return []resource.DiffEntry{{Path: logicalPath, Operation: "noop"}}, nil
}
func (r *testReconciler) Template(_ context.Context, _ string, value resource.Value) (resource.Value, error) {
	return value, nil
}

func isDirectChildPath(basePath string, candidatePath string) bool {
	base := path.Clean(basePath)
	candidate := path.Clean(candidatePath)
	if base == candidate {
		return true
	}
	if base == "/" {
		remaining := strings.TrimPrefix(candidate, "/")
		return remaining != "" && !strings.Contains(remaining, "/")
	}

	basePrefix := strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(candidate, basePrefix+"/") {
		return false
	}

	remaining := strings.TrimPrefix(candidate, basePrefix+"/")
	return remaining != "" && !strings.Contains(remaining, "/")
}

type testMetadata struct {
	items map[string]metadatadomain.ResourceMetadata
}

func newTestMetadata() *testMetadata {
	return &testMetadata{
		items: map[string]metadatadomain.ResourceMetadata{
			"/customers/acme": {
				IDFromAttribute: "id",
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationGet):     {Path: "/api/customers/acme"},
					string(metadatadomain.OperationCompare): {Path: "/api/customers/acme"},
				},
			},
		},
	}
}

func (s *testMetadata) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	metadata, found := s.items[logicalPath]
	if !found {
		return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
	}
	return metadata, nil
}

func (s *testMetadata) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	s.items[logicalPath] = metadata
	return nil
}

func (s *testMetadata) Unset(_ context.Context, logicalPath string) error {
	delete(s.items, logicalPath)
	return nil
}

func (s *testMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if metadata, found := s.items[logicalPath]; found {
		return metadata, nil
	}
	return metadatadomain.ResourceMetadata{}, nil
}

func (s *testMetadata) RenderOperationSpec(
	ctx context.Context,
	logicalPath string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	metadata, err := s.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	return metadatadomain.ResolveOperationSpec(ctx, metadata, operation, value)
}

func (s *testMetadata) Infer(
	ctx context.Context,
	logicalPath string,
	request metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.InferFromOpenAPI(ctx, logicalPath, request)
}

type testSecretProvider struct {
	values map[string]string
}

func newTestSecretProvider() *testSecretProvider {
	return &testSecretProvider{
		values: map[string]string{},
	}
}

func (s *testSecretProvider) Init(context.Context) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	return nil
}

func (s *testSecretProvider) Store(_ context.Context, key string, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *testSecretProvider) Get(_ context.Context, key string) (string, error) {
	value, found := s.values[key]
	if !found {
		return "", faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("secret %q not found", key), nil)
	}
	return value, nil
}

func (s *testSecretProvider) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func (s *testSecretProvider) List(context.Context) ([]string, error) {
	keys := make([]string, 0, len(s.values))
	for key := range s.values {
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *testSecretProvider) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return s.Store(ctx, key, secretValue)
	})
}

func (s *testSecretProvider) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return s.Get(ctx, key)
	})
}

func (s *testSecretProvider) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (s *testSecretProvider) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

type testRepository struct {
	deleteCalls []deleteCall
}

func (r *testRepository) Save(context.Context, string, resource.Value) error { return nil }
func (r *testRepository) Get(context.Context, string) (resource.Value, error) {
	return map[string]any{"id": "acme"}, nil
}
func (r *testRepository) Delete(_ context.Context, logicalPath string, policy repository.DeletePolicy) error {
	r.deleteCalls = append(r.deleteCalls, deleteCall{
		logicalPath: logicalPath,
		recursive:   policy.Recursive,
	})
	return nil
}
func (r *testRepository) List(_ context.Context, logicalPath string, policy repository.ListPolicy) ([]resource.Resource, error) {
	if policy.Recursive {
		return []resource.Resource{{LogicalPath: logicalPath + "/nested"}}, nil
	}
	return []resource.Resource{{LogicalPath: logicalPath}}, nil
}
func (r *testRepository) Exists(context.Context, string) (bool, error)        { return true, nil }
func (r *testRepository) Move(context.Context, string, string) error          { return nil }
func (r *testRepository) Init(context.Context) error                          { return nil }
func (r *testRepository) Refresh(context.Context) error                       { return nil }
func (r *testRepository) Reset(context.Context, repository.ResetPolicy) error { return nil }
func (r *testRepository) Check(context.Context) error                         { return nil }
func (r *testRepository) Push(context.Context, repository.PushPolicy) error   { return nil }
func (r *testRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
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
