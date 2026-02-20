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
		"config print-template",
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
	if !strings.Contains(debugOutput, `debug: root flags context="" output="auto" verbose=false no_status=false command="declarest resource get"`) {
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

	t.Run("repository_masks_metadata_declared_secret_by_default", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getLocalValues = map[string]resource.Value{
			"/customers/acme": map[string]any{
				"id":       "acme",
				"password": "plain-secret",
			},
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"password"},
		}

		output, err := executeForTest(deps, "", "resource", "get", "/customers/acme", "--repository")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, `"password": "{{secret .}}"`) {
			t.Fatalf("expected masked password in output, got %q", output)
		}
		if strings.Contains(output, "plain-secret") {
			t.Fatalf("expected plaintext secret to be hidden, got %q", output)
		}
	})

	t.Run("remote_server_masks_metadata_declared_secret_by_default", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteValue = map[string]any{
			"id":       "acme",
			"password": "plain-secret",
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"password"},
		}

		output, err := executeForTest(deps, "", "resource", "get", "/customers/acme", "--remote-server")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, `"password": "{{secret .}}"`) {
			t.Fatalf("expected masked password in output, got %q", output)
		}
		if strings.Contains(output, "plain-secret") {
			t.Fatalf("expected plaintext secret to be hidden, got %q", output)
		}
	})

	t.Run("show_secrets_flag_preserves_plaintext_for_repository_source", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getLocalValues = map[string]resource.Value{
			"/customers/acme": map[string]any{
				"id":       "acme",
				"password": "plain-secret",
			},
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"password"},
		}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/customers/acme",
			"--repository",
			"--show-secrets",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, `"password": "plain-secret"`) {
			t.Fatalf("expected plaintext password in output with --show-secrets, got %q", output)
		}
		if strings.Contains(output, `{{secret .}}`) {
			t.Fatalf("expected placeholder output to be disabled with --show-secrets, got %q", output)
		}
	})

	t.Run("show_secrets_flag_resolves_repository_placeholders_from_secret_store", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getLocalValues = map[string]resource.Value{
			"/customers/acme": map[string]any{
				"id":       "acme",
				"password": "{{secret .}}",
			},
		}

		secretProvider := deps.Secrets.(*testSecretProvider)
		if err := secretProvider.Store(context.Background(), "/customers/acme:password", "stored-secret"); err != nil {
			t.Fatalf("unexpected secret store setup error: %v", err)
		}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/customers/acme",
			"--repository",
			"--show-secrets",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, `"password": "stored-secret"`) {
			t.Fatalf("expected resolved stored secret in output with --show-secrets, got %q", output)
		}
		if strings.Contains(output, `{{secret .}}`) {
			t.Fatalf("expected placeholder output to be resolved with --show-secrets, got %q", output)
		}
	})

	t.Run("show_secrets_flag_requires_secret_provider_when_placeholders_exist", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getLocalValues = map[string]resource.Value{
			"/customers/acme": map[string]any{
				"id":       "acme",
				"password": "{{secret .}}",
			},
		}
		deps.Secrets = nil

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/customers/acme",
			"--repository",
			"--show-secrets",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "requires a configured secret provider") {
			t.Fatalf("expected secret provider validation error, got %v", err)
		}
	})

	t.Run("show_secrets_flag_preserves_plaintext_for_remote_source", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteValue = map[string]any{
			"id":       "acme",
			"password": "plain-secret",
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"password"},
		}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/customers/acme",
			"--remote-server",
			"--show-secrets",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, `"password": "plain-secret"`) {
			t.Fatalf("expected plaintext password in output with --show-secrets, got %q", output)
		}
		if strings.Contains(output, `{{secret .}}`) {
			t.Fatalf("expected placeholder output to be disabled with --show-secrets, got %q", output)
		}
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
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.adHocErr = faults.NewTypedError(faults.NotFoundError, "ad-hoc path not found", nil)
		orchestrator.getRemoteValue = map[string]any{
			"id":       "f88c68f3-3253-49f9-94a9-fe7553d33b5c",
			"clientId": "account",
		}

		output, err := executeForTest(deps, "", "ad-hoc", "get", "/admin/realms/master/clients/account")
		if err != nil {
			t.Fatalf("unexpected fallback error: %v", err)
		}
		if len(orchestrator.getRemoteCalls) != 1 {
			t.Fatalf("expected one metadata-aware fallback read, got %d", len(orchestrator.getRemoteCalls))
		}
		if orchestrator.getRemoteCalls[0] != "/admin/realms/master/clients/account" {
			t.Fatalf("expected fallback path to match request, got %q", orchestrator.getRemoteCalls[0])
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
		if output != "" {
			t.Fatalf("expected post output to be empty without --verbose, got %q", output)
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
		if output != "" {
			t.Fatalf("expected put output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("post_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "post", "/items", "--payload", `{"id":"a","name":"gamma"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected post output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("put_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "put", "/items/a", "--payload", `{"id":"a","name":"delta"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected put output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("post_verbose_renders_response_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), `{"id":"a","name":"alpha"}`, "ad-hoc", "post", "/items", "--verbose")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"POST\"") {
			t.Fatalf("expected POST method output with --verbose, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"alpha\"") {
			t.Fatalf("expected stdin payload to be forwarded with --verbose, got %q", output)
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
		if output != "" {
			t.Fatalf("expected delete output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("delete_with_force_verbose", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "ad-hoc", "delete", "/items/a", "--force", "--verbose")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"method\": \"DELETE\"") {
			t.Fatalf("expected DELETE method output with --verbose, got %q", output)
		}
		if !strings.Contains(output, "\"path\": \"/items/a\"") {
			t.Fatalf("expected delete path output with --verbose, got %q", output)
		}
	})

	t.Run("delete_collection_path_targets_direct_children", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/items/a"},
				{LogicalPath: "/items/b"},
				{LogicalPath: "/items/nested/c"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "ad-hoc", "delete", "/items", "--force")
		if err != nil {
			t.Fatalf("unexpected collection delete error: %v", err)
		}
		if len(orchestrator.adHocCalls) != 2 {
			t.Fatalf("expected 2 ad-hoc delete calls, got %#v", orchestrator.adHocCalls)
		}
		if orchestrator.adHocCalls[0].path != "/items/a" || orchestrator.adHocCalls[1].path != "/items/b" {
			t.Fatalf("expected direct-child delete paths [/items/a /items/b], got %#v", orchestrator.adHocCalls)
		}
		if output != "" {
			t.Fatalf("expected non-recursive delete output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("delete_collection_path_recursive_targets_full_tree", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/items/a"},
				{LogicalPath: "/items/b"},
				{LogicalPath: "/items/nested/c"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "ad-hoc", "delete", "/items", "--force", "--recursive")
		if err != nil {
			t.Fatalf("unexpected recursive collection delete error: %v", err)
		}
		if len(orchestrator.adHocCalls) != 3 {
			t.Fatalf("expected 3 ad-hoc delete calls, got %#v", orchestrator.adHocCalls)
		}
		if orchestrator.adHocCalls[2].path != "/items/nested/c" {
			t.Fatalf("expected recursive delete to include nested path, got %#v", orchestrator.adHocCalls)
		}
		if output != "" {
			t.Fatalf("expected recursive delete output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("delete_collection_path_recursive_verbose_renders_response_bodies", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/items/a"},
				{LogicalPath: "/items/b"},
				{LogicalPath: "/items/nested/c"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "ad-hoc", "delete", "/items", "--force", "--recursive", "--verbose")
		if err != nil {
			t.Fatalf("unexpected recursive collection delete error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/items/nested/c\"") {
			t.Fatalf("expected recursive delete output to include nested path with --verbose, got %q", output)
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected save path /customers/acme, got %q", orchestrator.saveCalls[0].logicalPath)
		}
		saved, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", orchestrator.saveCalls[0].value)
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
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a-id", "enabled": true},
				map[string]any{"id": "app-b-id", "enabled": false},
			},
		}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/master/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/admin/realms/master/clients/app-a-id" {
			t.Fatalf("expected first saved path to use id fallback, got %q", orchestrator.saveCalls[0].logicalPath)
		}
		if orchestrator.saveCalls[1].logicalPath != "/admin/realms/master/clients/app-b-id" {
			t.Fatalf("expected second saved path to use id fallback, got %q", orchestrator.saveCalls[1].logicalPath)
		}
	})

	t.Run("default_list_payload_saves_as_items", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`[{"id":"acme","tier":"pro"},{"id":"beta","tier":"free"}]`,
			"resource",
			"save",
			"/customers",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected first saved path /customers/acme, got %q", orchestrator.saveCalls[0].logicalPath)
		}
		if orchestrator.saveCalls[1].logicalPath != "/customers/beta" {
			t.Fatalf("expected second saved path /customers/beta, got %q", orchestrator.saveCalls[1].logicalPath)
		}
	})

	t.Run("as_one_resource_overrides_list_item_save", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`[{"id":"acme"},{"id":"beta"}]`,
			"resource",
			"save",
			"/customers",
			"--as-one-resource",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/customers" {
			t.Fatalf("expected saved path /customers, got %q", orchestrator.saveCalls[0].logicalPath)
		}
		if _, ok := orchestrator.saveCalls[0].value.([]any); !ok {
			t.Fatalf("expected single saved payload to be list, got %T", orchestrator.saveCalls[0].value)
		}
	})

	t.Run("as_items_requires_list_payload", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "--ignore") {
			t.Fatalf("expected --ignore hint, got %q", err.Error())
		}
		if len(orchestrator.saveCalls) != 0 {
			t.Fatalf("expected no save calls after safety failure, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("list_save_blocks_all_items_before_any_write_when_plaintext_secret_is_detected", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`[{"id":"acme","tier":"pro"},{"id":"beta","password":"plain-secret"}]`,
			"resource",
			"save",
			"/customers",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if len(orchestrator.saveCalls) != 0 {
			t.Fatalf("expected no partial writes when safety check fails, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("list_save_metadata_declared_plaintext_secret_is_auto_masked_and_stored", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:       "id",
			SecretsFromAttributes: []string{"secret"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)
		_, err := executeForTest(
			deps,
			`[{"id":"acme","secret":"plain-secret"}]`,
			"resource",
			"save",
			"/customers",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}

		savedPayload, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		if got := savedPayload["secret"]; got != `{{secret .}}` {
			t.Fatalf("expected saved secret placeholder, got %#v", got)
		}
		if secretProvider.values["/customers/acme:secret"] != "plain-secret" {
			t.Fatalf("expected stored secret value, got %#v", secretProvider.values)
		}
	})

	t.Run("metadata_secrets_from_attributes_auto_masks_and_stores_plaintext", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"credentials.authValue"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)
		_, err := executeForTest(
			deps,
			`{"credentials":{"authValue":"plain-secret"}}`,
			"resource",
			"save",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}

		savedPayload, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		credentials, ok := savedPayload["credentials"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested credentials map, got %T", savedPayload["credentials"])
		}
		if got := credentials["authValue"]; got != `{{secret .}}` {
			t.Fatalf("expected masked authValue placeholder, got %#v", got)
		}
		if secretProvider.values["/customers/acme:credentials.authValue"] != "plain-secret" {
			t.Fatalf("expected stored metadata-declared secret, got %#v", secretProvider.values)
		}
	})

	t.Run("metadata_secrets_from_attributes_requires_secret_provider", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"credentials.authValue"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		deps.Secrets = nil
		_, err := executeForTest(
			deps,
			`{"credentials":{"authValue":"plain-secret"}}`,
			"resource",
			"save",
			"/customers/acme",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "secret provider is not configured") {
			t.Fatalf("expected missing secret provider error, got %q", err.Error())
		}
		if len(orchestrator.saveCalls) != 0 {
			t.Fatalf("expected no save calls when secret provider is missing, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("metadata_secrets_from_attributes_accepts_placeholders", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"credentials.authValue"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`{"credentials":{"authValue":"{{secret \"authValue\"}}"}}`,
			"resource",
			"save",
			"/customers/acme",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("ignore_flag_allows_plaintext_secret", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
			"--ignore",
		)
		if err != nil {
			t.Fatalf("unexpected error with --ignore: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("ignore_flag_allows_metadata_declared_secret", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			SecretsFromAttributes: []string{"password"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`{"password":"plain-secret"}`,
			"resource",
			"save",
			"/customers/acme",
			"--ignore",
		)
		if err != nil {
			t.Fatalf("unexpected error with --ignore: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}
	})

	t.Run("handle_secrets_masks_payload_updates_metadata_and_stores_secret_values", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:       "id",
			SecretsFromAttributes: []string{"credentials.authValue", "existingSecret"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)
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
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 save call, got %d", len(orchestrator.saveCalls))
		}

		saved, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		if got := saved["apiToken"]; got != `{{secret .}}` {
			t.Fatalf("expected apiToken placeholder, got %#v", got)
		}
		credentials, ok := saved["credentials"].(map[string]any)
		if !ok {
			t.Fatalf("expected nested credentials payload, got %T", saved["credentials"])
		}
		if got := credentials["authValue"]; got != `{{secret .}}` {
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
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)
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
		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}

		firstSaved, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected first saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		secondSaved, ok := orchestrator.saveCalls[1].value.(map[string]any)
		if !ok {
			t.Fatalf("expected second saved payload map, got %T", orchestrator.saveCalls[1].value)
		}
		if got := firstSaved["password"]; got != `{{secret .}}` {
			t.Fatalf("expected first path-scoped placeholder, got %#v", got)
		}
		if got := secondSaved["password"]; got != `{{secret .}}` {
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
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)
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
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)
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
		if len(orchestrator.saveCalls) != 0 {
			t.Fatalf("expected no save calls when unhandled secrets remain, got %d", len(orchestrator.saveCalls))
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
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
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a", "secret": "sec-a", "apiToken": "tok-a"},
				map[string]any{"id": "app-b", "secret": "sec-b", "apiToken": "tok-b"},
			},
		}
		deps := newResourceSaveDeps(orchestrator, metadataService)
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
		if len(orchestrator.saveCalls) != 0 {
			t.Fatalf("expected no save calls when unhandled secrets remain, got %d", len(orchestrator.saveCalls))
		}

		wildcardMetadata := metadataService.items["/admin/realms/_/clients"]
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
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			getRemoteValue: []any{
				map[string]any{"id": "app-a", "secret": "sec-a", "apiToken": "tok-a"},
				map[string]any{"id": "app-b", "secret": "sec-b", "apiToken": "tok-b"},
			},
		}
		deps := newResourceSaveDeps(orchestrator, metadataService)

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
		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}

		firstPayload, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected first saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		if got := firstPayload["secret"]; got != `{{secret .}}` {
			t.Fatalf("expected first secret placeholder, got %#v", got)
		}
		if got := firstPayload["apiToken"]; got != "tok-a" {
			t.Fatalf("expected unhandled apiToken to remain plaintext, got %#v", got)
		}

		wildcardMetadata := metadataService.items["/admin/realms/_/clients"]
		if !reflect.DeepEqual(wildcardMetadata.SecretsFromAttributes, []string{"secret"}) {
			t.Fatalf("expected wildcard metadata secretsFromAttributes to include secret, got %#v", wildcardMetadata.SecretsFromAttributes)
		}
	})

	t.Run("wildcard_handle_secrets_requested_attribute_skips_collections_without_candidate", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/clients"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		metadataService.items["/admin/realms/tenant-a/clients"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			remoteList: []resource.Resource{
				{LogicalPath: "/admin/realms/master"},
				{LogicalPath: "/admin/realms/tenant-a"},
			},
			getRemoteValues: map[string]resource.Value{
				"/admin/realms/master/clients": []any{
					map[string]any{"id": "app-a", "secret": "sec-a"},
				},
				"/admin/realms/tenant-a/clients": []any{
					map[string]any{"id": "app-b", "enabled": true},
				},
			},
		}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		secretProvider := deps.Secrets.(*testSecretProvider)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/_/clients",
			"--handle-secrets=secret",
		)
		if err != nil {
			t.Fatalf("unexpected wildcard save error with --handle-secrets=secret: %v", err)
		}
		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}

		savedByPath := make(map[string]resource.Value, len(orchestrator.saveCalls))
		for _, call := range orchestrator.saveCalls {
			savedByPath[call.logicalPath] = call.value
		}

		masterPayload, ok := savedByPath["/admin/realms/master/clients/app-a"].(map[string]any)
		if !ok {
			t.Fatalf("expected saved master payload map, got %T", savedByPath["/admin/realms/master/clients/app-a"])
		}
		if got := masterPayload["secret"]; got != `{{secret .}}` {
			t.Fatalf("expected master secret placeholder, got %#v", got)
		}

		tenantPayload, ok := savedByPath["/admin/realms/tenant-a/clients/app-b"].(map[string]any)
		if !ok {
			t.Fatalf("expected saved tenant payload map, got %T", savedByPath["/admin/realms/tenant-a/clients/app-b"])
		}
		if got, found := tenantPayload["secret"]; found {
			t.Fatalf("expected tenant payload to skip secret handling, got %#v", got)
		}
		if got := tenantPayload["enabled"]; got != true {
			t.Fatalf("expected tenant payload preserved, got %#v", got)
		}

		if secretProvider.values["/admin/realms/master/clients/app-a:secret"] != "sec-a" {
			t.Fatalf("expected master secret to be stored, got %#v", secretProvider.values)
		}
		if _, found := secretProvider.values["/admin/realms/tenant-a/clients/app-b:secret"]; found {
			t.Fatalf("expected tenant secret key to be absent, got %#v", secretProvider.values)
		}
	})
}

func TestResourceSaveWildcardPaths(t *testing.T) {
	t.Parallel()

	t.Run("collection_wildcard_saves_items_from_all_matched_collections", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			remoteList: []resource.Resource{
				{LogicalPath: "/admin/realms/master"},
				{LogicalPath: "/admin/realms/tenant-a"},
			},
			getRemoteValues: map[string]resource.Value{
				"/admin/realms/master/clients": []any{
					map[string]any{"id": "master-client", "enabled": true},
				},
				"/admin/realms/tenant-a/clients": []any{
					map[string]any{"id": "tenant-client", "enabled": true},
				},
			},
		}
		deps := newResourceSaveDeps(orchestrator, metadataService)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/_/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected wildcard collection save error: %v", err)
		}

		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/admin/realms/master/clients/master-client" &&
			orchestrator.saveCalls[1].logicalPath != "/admin/realms/master/clients/master-client" {
			t.Fatalf("expected master collection item to be saved, got %#v", orchestrator.saveCalls)
		}
		if orchestrator.saveCalls[0].logicalPath != "/admin/realms/tenant-a/clients/tenant-client" &&
			orchestrator.saveCalls[1].logicalPath != "/admin/realms/tenant-a/clients/tenant-client" {
			t.Fatalf("expected tenant collection item to be saved, got %#v", orchestrator.saveCalls)
		}
	})

	t.Run("resource_wildcard_saves_only_existing_matches", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			remoteList: []resource.Resource{
				{LogicalPath: "/admin/realms/master"},
				{LogicalPath: "/admin/realms/tenant-a"},
			},
			getRemoteValues: map[string]resource.Value{
				"/admin/realms/master/clients/test": map[string]any{"id": "test", "realm": "master"},
			},
		}
		deps := newResourceSaveDeps(orchestrator, metadataService)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/_/clients/test",
		)
		if err != nil {
			t.Fatalf("unexpected wildcard resource save error: %v", err)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected 1 saved match, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/admin/realms/master/clients/test" {
			t.Fatalf("expected /admin/realms/master/clients/test save path, got %q", orchestrator.saveCalls[0].logicalPath)
		}
	})

	t.Run("wildcard_path_rejects_inline_payload", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)

		_, err := executeForTest(
			deps,
			`{"id":"test"}`,
			"resource",
			"save",
			"/admin/realms/_/clients/test",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "wildcard save paths") {
			t.Fatalf("expected wildcard payload validation error, got %q", err.Error())
		}
	})

	t.Run("wildcard_path_returns_not_found_when_no_remote_matches_exist", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := newResourceSaveDeps(orchestrator, metadataService)

		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/_/clients",
		)
		assertTypedCategory(t, err, faults.NotFoundError)
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
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(orchestrator.deleteCalls) != 1 {
			t.Fatalf("expected 1 remote delete call, got %d", len(orchestrator.deleteCalls))
		}
		if len(repositoryService.deleteCalls) != 0 {
			t.Fatalf("expected 0 repository delete calls, got %d", len(repositoryService.deleteCalls))
		}
	})

	t.Run("repository_flag_deletes_only_from_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force", "--repository")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(orchestrator.deleteCalls) != 0 {
			t.Fatalf("expected 0 remote delete calls, got %d", len(orchestrator.deleteCalls))
		}
		if len(repositoryService.deleteCalls) != 1 {
			t.Fatalf("expected 1 repository delete call, got %d", len(repositoryService.deleteCalls))
		}
	})

	t.Run("both_flag_deletes_from_remote_and_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.Repository.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--force", "--both")
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(orchestrator.deleteCalls) != 1 {
			t.Fatalf("expected 1 remote delete call, got %d", len(orchestrator.deleteCalls))
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

func TestResourceDeleteCollectionPathUsesRepositoryTargetsForRemoteDelete(t *testing.T) {
	t.Parallel()

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
			{LogicalPath: "/customers/beta"},
			{LogicalPath: "/customers/nested/gamma"},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	_, err := executeForTest(deps, "", "resource", "delete", "/customers", "--force")
	if err != nil {
		t.Fatalf("unexpected non-recursive delete error: %v", err)
	}
	if len(orchestrator.deleteCalls) != 2 {
		t.Fatalf("expected 2 remote delete calls for non-recursive collection delete, got %d", len(orchestrator.deleteCalls))
	}
	if orchestrator.deleteCalls[0].logicalPath != "/customers/acme" || orchestrator.deleteCalls[1].logicalPath != "/customers/beta" {
		t.Fatalf(
			"expected non-recursive remote delete paths [/customers/acme /customers/beta], got [%s %s]",
			orchestrator.deleteCalls[0].logicalPath,
			orchestrator.deleteCalls[1].logicalPath,
		)
	}
	if orchestrator.deleteCalls[0].recursive || orchestrator.deleteCalls[1].recursive {
		t.Fatalf("expected expanded non-recursive delete targets to use recursive=false, got %#v", orchestrator.deleteCalls)
	}

	orchestrator.deleteCalls = nil
	_, err = executeForTest(deps, "", "resource", "delete", "/customers", "--force", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive delete error: %v", err)
	}
	if len(orchestrator.deleteCalls) != 3 {
		t.Fatalf("expected 3 remote delete calls for recursive collection delete, got %d", len(orchestrator.deleteCalls))
	}
	if orchestrator.deleteCalls[2].logicalPath != "/customers/nested/gamma" {
		t.Fatalf("expected recursive delete to include nested path, got %#v", orchestrator.deleteCalls)
	}
	for _, call := range orchestrator.deleteCalls {
		if call.recursive {
			t.Fatalf("expected expanded recursive delete targets to execute as single-resource deletes, got %#v", orchestrator.deleteCalls)
		}
	}
}

func TestResourceDeleteFallsBackToRequestedPathWhenNoLocalTargetsMatch(t *testing.T) {
	t.Parallel()

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	_, err := executeForTest(deps, "", "resource", "delete", "/orders", "--force", "--recursive")
	if err != nil {
		t.Fatalf("unexpected delete fallback error: %v", err)
	}
	if len(orchestrator.deleteCalls) != 1 {
		t.Fatalf("expected one fallback delete call, got %#v", orchestrator.deleteCalls)
	}
	if orchestrator.deleteCalls[0].logicalPath != "/orders" {
		t.Fatalf("expected fallback delete path /orders, got %q", orchestrator.deleteCalls[0].logicalPath)
	}
	if !orchestrator.deleteCalls[0].recursive {
		t.Fatalf("expected fallback delete to preserve recursive=true policy")
	}
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

	t.Run("render_flag_path_without_operation_uses_default", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "metadata", "render", "--path", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected render default-operation error with --path: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/api/customers/acme\"") {
			t.Fatalf("expected default get render output with --path, got %q", output)
		}
	})

	t.Run("render_mismatch_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "metadata", "render", "/customers/a", "--path", "/customers/b", "get")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("render_missing_operation", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "metadata", "render", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected render default-operation error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/api/customers/acme\"") {
			t.Fatalf("expected default get render output, got %q", output)
		}
	})

	t.Run("render_missing_operation_falls_back_to_list_when_get_path_is_missing", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms"] = metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationList): {
					Method: "GET",
					Path:   "/admin/realms",
				},
			},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(testDepsWith(orchestrator, metadataService), "", "metadata", "render", "/admin/realms")
		if err != nil {
			t.Fatalf("unexpected render fallback error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/admin/realms\"") {
			t.Fatalf("expected fallback list render path, got %q", output)
		}
	})

	t.Run("render_collection_selector_defaults_to_list", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/_/clients/"] = metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationList): {
					Path: "/admin/realms/{{.realm}}/clients",
				},
			},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(testDepsWith(orchestrator, metadataService), "", "metadata", "render", "/admin/realms/_/clients/")
		if err != nil {
			t.Fatalf("unexpected selector render default-operation error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/admin/realms/{{.realm}}/clients\"") {
			t.Fatalf("expected list render output for selector path, got %q", output)
		}
	})

	t.Run("infer_collection_selector_uses_openapi_and_omits_null_fields", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms/{realm}/clients": map[string]any{
						"get":  map[string]any{},
						"post": map[string]any{},
					},
					"/admin/realms/{realm}/clients/{clientId}": map[string]any{
						"get":    map[string]any{},
						"put":    map[string]any{},
						"delete": map[string]any{},
					},
				},
			},
		}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"infer",
			"/admin/realms/_/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected infer selector error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"id\"") {
			t.Fatalf("expected inferred idFromAttribute, got %q", output)
		}
		if !strings.Contains(output, "\"aliasFromAttribute\": \"clientId\"") {
			t.Fatalf("expected inferred aliasFromAttribute, got %q", output)
		}
		if !strings.Contains(output, "\"secretInAttributes\": [\n      \"secret\"\n    ]") {
			t.Fatalf("expected inferred secretInAttributes, got %q", output)
		}
		if strings.Contains(output, "\"operations\": null") ||
			strings.Contains(output, "\"filter\": null") ||
			strings.Contains(output, "\"suppress\": null") {
			t.Fatalf("expected infer output without null metadata fields, got %q", output)
		}
		if strings.Contains(output, "\"operations\"") {
			t.Fatalf("expected openapi-default operations to be omitted from infer output, got %q", output)
		}
	})

	t.Run("infer_collection_selector_omits_default_operations_with_non_template_safe_openapi_param", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms/{realm}/clients": map[string]any{
						"get":  map[string]any{},
						"post": map[string]any{},
					},
					"/admin/realms/{realm}/clients/{client-uuid}": map[string]any{
						"get":    map[string]any{},
						"put":    map[string]any{},
						"delete": map[string]any{},
					},
				},
			},
		}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"infer",
			"/admin/realms/_/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected infer selector error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"id\"") {
			t.Fatalf("expected inferred idFromAttribute, got %q", output)
		}
		if !strings.Contains(output, "\"aliasFromAttribute\": \"clientId\"") {
			t.Fatalf("expected inferred aliasFromAttribute, got %q", output)
		}
		if !strings.Contains(output, "\"secretInAttributes\": [\n      \"secret\"\n    ]") {
			t.Fatalf("expected inferred secretInAttributes, got %q", output)
		}
		if strings.Contains(output, "\"operations\"") {
			t.Fatalf("expected openapi-default operations to be omitted from infer output, got %q", output)
		}
	})

	t.Run("infer_collection_path_uses_openapi_identity_and_omits_default_operations", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms": map[string]any{
						"get":  map[string]any{},
						"post": map[string]any{},
					},
					"/admin/realms/{realm}": map[string]any{
						"get":    map[string]any{},
						"put":    map[string]any{},
						"delete": map[string]any{},
					},
				},
			},
		}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"infer",
			"/admin/realms",
		)
		if err != nil {
			t.Fatalf("unexpected infer collection-path error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"realm\"") {
			t.Fatalf("expected inferred idFromAttribute=realm, got %q", output)
		}
		if !strings.Contains(output, "\"aliasFromAttribute\": \"realm\"") {
			t.Fatalf("expected inferred aliasFromAttribute=realm, got %q", output)
		}
		if strings.Contains(output, "\"operations\"") {
			t.Fatalf("expected default operations to be omitted from infer output, got %q", output)
		}
	})

	t.Run("get_omits_nil_fields", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/_/clients/"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:       "id",
			AliasFromAttribute:    "clientId",
			SecretsFromAttributes: []string{"secret"},
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/_/clients/",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get error: %v", err)
		}
		if strings.Contains(output, "\"operations\": null") ||
			strings.Contains(output, "\"filter\": null") ||
			strings.Contains(output, "\"suppress\": null") {
			t.Fatalf("expected metadata get output without null fields, got %q", output)
		}
	})

	t.Run("get_not_found_falls_back_to_infer_when_openapi_has_endpoint", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		delete(metadataService.items, "/admin/realms/")
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms": map[string]any{
						"get":  map[string]any{},
						"post": map[string]any{},
					},
					"/admin/realms/{realm}": map[string]any{
						"get":    map[string]any{},
						"put":    map[string]any{},
						"delete": map[string]any{},
					},
				},
			},
		}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get fallback error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"realm\"") {
			t.Fatalf("expected inferred idFromAttribute in metadata get fallback output, got %q", output)
		}
		if !strings.Contains(output, "\"aliasFromAttribute\": \"realm\"") {
			t.Fatalf("expected inferred aliasFromAttribute in metadata get fallback output, got %q", output)
		}
	})

	t.Run("get_not_found_without_openapi_or_remote_keeps_not_found", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		delete(metadataService.items, "/admin/realms/")
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec:     nil,
			listRemoteErr: faults.NewTypedError(
				faults.NotFoundError,
				"resource \"/admin/realms/\" not found",
				nil,
			),
		}

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/",
		)
		assertTypedCategory(t, err, faults.NotFoundError)
	})

	t.Run("infer_apply_persists_compact_metadata_without_defaults", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms": map[string]any{
						"get":  map[string]any{},
						"post": map[string]any{},
					},
					"/admin/realms/{realm}": map[string]any{
						"get":    map[string]any{},
						"put":    map[string]any{},
						"delete": map[string]any{},
					},
				},
			},
		}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"infer",
			"/admin/realms/",
			"--apply",
		)
		if err != nil {
			t.Fatalf("unexpected infer apply error: %v", err)
		}
		if !strings.HasSuffix(output, "\n") {
			t.Fatalf("expected infer output to end with newline, got %q", output)
		}
		if strings.Contains(output, "\"operations\"") {
			t.Fatalf("expected infer apply output without default operations, got %q", output)
		}

		stored, found := metadataService.items["/admin/realms/"]
		if !found {
			t.Fatal("expected inferred metadata to be persisted")
		}
		if stored.IDFromAttribute != "realm" || stored.AliasFromAttribute != "realm" {
			t.Fatalf("expected persisted compact identity attributes, got %#v", stored)
		}
		if len(stored.Operations) != 0 {
			t.Fatalf("expected persisted metadata without default operations, got %#v", stored.Operations)
		}
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

	t.Run("get_accepts_path_key_input_formats_and_outputs_plaintext", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		seedSecrets := []struct {
			key   string
			value string
		}{
			{key: "/customers/acme:apiToken", value: "token-123"},
			{key: "/customers/acme:password", value: "pw-123"},
			{key: "/customers/acme:quoted", value: `he said "hi"`},
			{key: "/customers/beta:apiToken", value: "token-b"},
		}
		for _, item := range seedSecrets {
			if _, err := executeForTest(deps, "", "secret", "store", item.key, item.value); err != nil {
				t.Fatalf("store %q returned error: %v", item.key, err)
			}
		}

		tests := []struct {
			name   string
			args   []string
			expect string
		}{
			{
				name:   "path_only_lists_all_for_path",
				args:   []string{"secret", "get", "/customers/acme"},
				expect: "apiToken=token-123\npassword=pw-123\nquoted=he said \"hi\"\n",
			},
			{
				name:   "path_and_key_positional",
				args:   []string{"secret", "get", "/customers/acme", "apiToken"},
				expect: "token-123\n",
			},
			{
				name:   "path_flag_only_lists_all_for_path",
				args:   []string{"secret", "get", "--path", "/customers/acme"},
				expect: "apiToken=token-123\npassword=pw-123\nquoted=he said \"hi\"\n",
			},
			{
				name:   "path_and_key_flags",
				args:   []string{"secret", "get", "--path", "/customers/acme", "--key", "apiToken"},
				expect: "token-123\n",
			},
			{
				name:   "composite_path_key",
				args:   []string{"secret", "get", "/customers/acme:apiToken"},
				expect: "token-123\n",
			},
		}

		for _, testCase := range tests {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				output, err := executeForTest(deps, "", testCase.args...)
				if err != nil {
					t.Fatalf("secret get returned error: %v", err)
				}
				if output != testCase.expect {
					t.Fatalf("unexpected output; expected %q, got %q", testCase.expect, output)
				}
				if strings.Contains(output, "\"token-123\"") || strings.Contains(output, "\"pw-123\"") {
					t.Fatalf("expected plain text output without added quotes, got %q", output)
				}
			})
		}
	})

	t.Run("get_path_and_key_preserves_quotes_when_secret_contains_quotes", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		if _, err := executeForTest(deps, "", "secret", "store", "/customers/acme:quoted", `he said "hi"`); err != nil {
			t.Fatalf("store returned error: %v", err)
		}

		output, err := executeForTest(deps, "", "secret", "get", "/customers/acme", "quoted")
		if err != nil {
			t.Fatalf("get returned error: %v", err)
		}
		if output != "he said \"hi\"\n" {
			t.Fatalf("expected quoted content preserved in plain text output, got %q", output)
		}
	})

	t.Run("get_with_key_flag_requires_path", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "secret", "get", "--key", "apiToken")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("get_ignores_structured_output_modes_and_stays_plain_text", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		if _, err := executeForTest(deps, "", "secret", "store", "/customers/acme:apiToken", "token-123"); err != nil {
			t.Fatalf("store returned error: %v", err)
		}

		output, err := executeForTest(deps, "", "--output", "json", "secret", "get", "/customers/acme:apiToken")
		if err != nil {
			t.Fatalf("get returned error: %v", err)
		}
		if output != "token-123\n" {
			t.Fatalf("expected plain text output even with --output json, got %q", output)
		}
	})

	t.Run("mask_and_resolve_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		payload := `{"apiToken":"token-abc","name":"acme"}`
		masked, err := executeForTest(deps, payload, "secret", "mask")
		if err != nil {
			t.Fatalf("mask returned error: %v", err)
		}
		if !strings.Contains(masked, `{{secret .}}`) {
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
		orchestrator := &testOrchestrator{
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

		output, err := executeForTest(testDepsWith(orchestrator, metadataService), "", "secret", "detect")
		if err != nil {
			t.Fatalf("detect without input returned error: %v", err)
		}
		if len(orchestrator.listLocalCalls) != 1 || orchestrator.listLocalCalls[0] != "/" {
			t.Fatalf("expected repo-wide scan with path \"/\", got %#v", orchestrator.listLocalCalls)
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
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
			},
		}

		output, err := executeForTest(testDepsWith(orchestrator, metadataService), "", "secret", "detect", "/customers")
		if err != nil {
			t.Fatalf("detect path scope without input returned error: %v", err)
		}
		if len(orchestrator.listLocalCalls) != 1 || orchestrator.listLocalCalls[0] != "/customers" {
			t.Fatalf("expected scoped scan with path \"/customers\", got %#v", orchestrator.listLocalCalls)
		}
		if !strings.Contains(output, "\"LogicalPath\": \"/customers/acme\"") {
			t.Fatalf("expected scoped detect output, got %q", output)
		}
	})

	t.Run("detect_fix_updates_metadata_for_target_path", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
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
		orchestrator := &testOrchestrator{
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

		_, err := executeForTest(testDepsWith(orchestrator, metadataService), "", "secret", "detect", "--fix")
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
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
		orchestrator := &testOrchestrator{metadataService: metadataService}

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
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
		orchestrator := &testOrchestrator{
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
			testDepsWith(orchestrator, metadataService),
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
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme": map[string]any{"password": "pw-123"},
			},
		}

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
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

	t.Run("filesystem_context_text_output", func(t *testing.T) {
		t.Parallel()

		textOutput, err := executeForTest(testDeps(), "", "repo", "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(textOutput, "type=filesystem sync=not_applicable") {
			t.Fatalf("expected filesystem text repo status output, got %q", textOutput)
		}
	})

	t.Run("git_context_text_output", func(t *testing.T) {
		t.Parallel()

		textOutput, err := executeForTest(testDeps(), "", "--context", "git", "repo", "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(textOutput, "type=git state=no_remote") {
			t.Fatalf("expected git text repo status output, got %q", textOutput)
		}
	})

	t.Run("git_context_without_remote_text_output", func(t *testing.T) {
		t.Parallel()

		textOutput, err := executeForTest(testDeps(), "", "--context", "git-no-remote", "repo", "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(textOutput, "type=git state=no_remote remote=not_configured") {
			t.Fatalf("expected git no-remote text repo status output, got %q", textOutput)
		}
	})

	t.Run("json_output_remains_structured", func(t *testing.T) {
		t.Parallel()

		jsonOutput, err := executeForTest(testDeps(), "", "-o", "json", "repo", "status")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(jsonOutput, "\"state\": \"no_remote\"") {
			t.Fatalf("expected structured json status output, got %q", jsonOutput)
		}
	})
}

func TestRepoPushTypeAwareValidation(t *testing.T) {
	t.Parallel()

	t.Run("filesystem_context_fails_fast", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "repo", "push")
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "filesystem repositories") {
			t.Fatalf("expected filesystem-specific validation error, got %v", err)
		}
	})

	t.Run("git_context_calls_push", func(t *testing.T) {
		t.Parallel()

		repoService := &testRepository{}
		deps := testDeps()
		deps.Repository = repoService

		if _, err := executeForTest(deps, "", "--context", "git", "repo", "push"); err != nil {
			t.Fatalf("unexpected push error: %v", err)
		}
		if repoService.pushCalls != 1 {
			t.Fatalf("expected push to be called once, got %d", repoService.pushCalls)
		}
	})

	t.Run("git_context_without_remote_fails_validation", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "--context", "git-no-remote", "repo", "push")
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "repository.git.remote") {
			t.Fatalf("expected git remote validation error, got %v", err)
		}
	})
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

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "resource", "list", "/")
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

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "resource", "list", "/", "--remote-server")
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

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "resource", "list", "/", "--repository")
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

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
			{LogicalPath: "/customers/beta"},
			{LogicalPath: "/customers/nested/gamma"},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	directOutput, err := executeForTest(deps, "", "resource", "apply", "/customers")
	if err != nil {
		t.Fatalf("unexpected direct apply error: %v", err)
	}
	expectedDirectCalls := []string{"/customers/acme", "/customers/beta"}
	if !reflect.DeepEqual(orchestrator.applyCalls, expectedDirectCalls) {
		t.Fatalf("expected direct apply calls %#v, got %#v", expectedDirectCalls, orchestrator.applyCalls)
	}
	if directOutput != "" {
		t.Fatalf("expected direct apply output to be empty without --verbose, got %q", directOutput)
	}

	orchestrator.applyCalls = nil
	recursiveOutput, err := executeForTest(deps, "", "resource", "apply", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive apply error: %v", err)
	}
	expectedRecursiveCalls := []string{"/customers/acme", "/customers/beta", "/customers/nested/gamma"}
	if !reflect.DeepEqual(orchestrator.applyCalls, expectedRecursiveCalls) {
		t.Fatalf("expected recursive apply calls %#v, got %#v", expectedRecursiveCalls, orchestrator.applyCalls)
	}
	if recursiveOutput != "" {
		t.Fatalf("expected recursive apply output to be empty without --verbose, got %q", recursiveOutput)
	}

	verboseOutput, err := executeForTest(deps, "", "resource", "apply", "/customers", "--recursive", "--verbose")
	if err != nil {
		t.Fatalf("unexpected recursive apply verbose error: %v", err)
	}
	if !strings.Contains(verboseOutput, "\"LogicalPath\": \"/customers/nested/gamma\"") {
		t.Fatalf("expected recursive apply output to include nested resource with --verbose, got %q", verboseOutput)
	}
}

func TestResourceCreateUsesExplicitOrRepositoryInput(t *testing.T) {
	t.Parallel()

	t.Run("create_with_input", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "create", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if len(orchestrator.createCalls) != 1 {
			t.Fatalf("expected single create call, got %d", len(orchestrator.createCalls))
		}
		if orchestrator.createCalls[0].logicalPath != "/customers/acme" {
			t.Fatalf("expected create path /customers/acme, got %q", orchestrator.createCalls[0].logicalPath)
		}
		if output != "" {
			t.Fatalf("expected create output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("create_without_input_uses_repository_targets", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
				{LogicalPath: "/customers/nested/gamma"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme":         map[string]any{"id": "acme", "tier": "pro"},
				"/customers/beta":         map[string]any{"id": "beta", "tier": "free"},
				"/customers/nested/gamma": map[string]any{"id": "gamma", "tier": "enterprise"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "create", "/customers")
		if err != nil {
			t.Fatalf("unexpected create collection error: %v", err)
		}
		if len(orchestrator.createCalls) != 2 {
			t.Fatalf("expected 2 create calls for non-recursive collection create, got %d", len(orchestrator.createCalls))
		}
		if orchestrator.createCalls[0].logicalPath != "/customers/acme" || orchestrator.createCalls[1].logicalPath != "/customers/beta" {
			t.Fatalf("expected non-recursive create paths [/customers/acme /customers/beta], got [%s %s]", orchestrator.createCalls[0].logicalPath, orchestrator.createCalls[1].logicalPath)
		}
		if len(orchestrator.getLocalCalls) != 2 {
			t.Fatalf("expected 2 local payload lookups, got %#v", orchestrator.getLocalCalls)
		}
		if !reflect.DeepEqual(orchestrator.createCalls[0].value, orchestrator.getLocalValues["/customers/acme"]) {
			t.Fatalf("expected create payload to come from local resource for /customers/acme")
		}
		if !reflect.DeepEqual(orchestrator.createCalls[1].value, orchestrator.getLocalValues["/customers/beta"]) {
			t.Fatalf("expected create payload to come from local resource for /customers/beta")
		}
		if output != "" {
			t.Fatalf("expected non-recursive create output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("create_without_input_recursive_includes_descendants", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
				{LogicalPath: "/customers/nested/gamma"},
			},
			getLocalValues: map[string]resource.Value{
				"/customers/acme":         map[string]any{"id": "acme"},
				"/customers/beta":         map[string]any{"id": "beta"},
				"/customers/nested/gamma": map[string]any{"id": "gamma"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "create", "/customers", "--recursive")
		if err != nil {
			t.Fatalf("unexpected recursive create error: %v", err)
		}
		if len(orchestrator.createCalls) != 3 {
			t.Fatalf("expected 3 create calls for recursive create, got %d", len(orchestrator.createCalls))
		}
		if output != "" {
			t.Fatalf("expected recursive create output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("create_without_input_fails_when_no_local_resources_match", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, "", "resource", "create", "/orders")
		assertTypedCategory(t, err, faults.NotFoundError)
	})

	t.Run("create_recursive_rejects_explicit_input", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, `{"id":"acme"}`, "resource", "create", "/customers/acme", "--recursive")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("create_with_payload_flag", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "create", "/customers/acme", "--payload", `{"id":"acme","tier":"startup"}`)
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if len(orchestrator.createCalls) != 1 {
			t.Fatalf("expected single create call, got %d", len(orchestrator.createCalls))
		}
		if output != "" {
			t.Fatalf("expected create output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("create_with_payload_flag_verbose_renders_target", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "create", "/customers/acme", "--payload", `{"id":"acme","tier":"startup"}`, "--verbose")
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if !strings.Contains(output, "/customers/acme") {
			t.Fatalf("expected create output to contain /customers/acme with --verbose, got %q", output)
		}
	})

	t.Run("create_payload_conflicts_with_file", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)
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

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

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

	orchestrator := &testOrchestrator{
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
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	output, err := executeForTest(deps, "", "resource", "update", "/customers")
	if err != nil {
		t.Fatalf("unexpected update collection error: %v", err)
	}
	if len(orchestrator.updateCalls) != 1 {
		t.Fatalf("expected 1 update call for non-recursive update, got %d", len(orchestrator.updateCalls))
	}
	if orchestrator.updateCalls[0].logicalPath != "/customers/acme" {
		t.Fatalf("expected non-recursive update path /customers/acme, got %q", orchestrator.updateCalls[0].logicalPath)
	}
	if len(orchestrator.getLocalCalls) != 1 || orchestrator.getLocalCalls[0] != "/customers/acme" {
		t.Fatalf("expected non-recursive update to read only /customers/acme, got %#v", orchestrator.getLocalCalls)
	}
	if !reflect.DeepEqual(orchestrator.updateCalls[0].value, orchestrator.getLocalValues["/customers/acme"]) {
		t.Fatalf("expected update payload to come from local resource for /customers/acme")
	}
	if output != "" {
		t.Fatalf("expected non-recursive update output to be empty without --verbose, got %q", output)
	}

	orchestrator.updateCalls = nil
	orchestrator.getLocalCalls = nil
	recursiveOutput, err := executeForTest(deps, "", "resource", "update", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive update error: %v", err)
	}
	if len(orchestrator.updateCalls) != 2 {
		t.Fatalf("expected 2 update calls for recursive update, got %d", len(orchestrator.updateCalls))
	}
	if recursiveOutput != "" {
		t.Fatalf("expected recursive update output to be empty without --verbose, got %q", recursiveOutput)
	}

	verboseOutput, err := executeForTest(deps, "", "resource", "update", "/customers", "--recursive", "--verbose")
	if err != nil {
		t.Fatalf("unexpected recursive update verbose error: %v", err)
	}
	if !strings.Contains(verboseOutput, "/customers/nested/gamma") {
		t.Fatalf("expected recursive update output to include nested resources with --verbose, got %q", verboseOutput)
	}
}

func TestResourceDiffCollectionPath(t *testing.T) {
	t.Parallel()

	t.Run("collection_path_compares_all_direct_children", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
				{LogicalPath: "/customers/beta"},
				{LogicalPath: "/customers/nested/gamma"},
			},
			diffValues: map[string][]resource.DiffEntry{
				"/customers/acme": {
					{Path: "/customers/acme/name", Operation: "replace"},
				},
				"/customers/beta": {
					{Path: "/customers/beta/enabled", Operation: "remove"},
				},
				"/customers/nested/gamma": {
					{Path: "/customers/nested/gamma/name", Operation: "replace"},
				},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "diff", "/customers")
		if err != nil {
			t.Fatalf("unexpected diff collection error: %v", err)
		}

		expectedCalls := []string{"/customers/acme", "/customers/beta"}
		if !reflect.DeepEqual(orchestrator.diffCalls, expectedCalls) {
			t.Fatalf("expected non-recursive diff calls %#v, got %#v", expectedCalls, orchestrator.diffCalls)
		}
		if !strings.Contains(output, "/customers/acme/name") || !strings.Contains(output, "/customers/beta/enabled") {
			t.Fatalf("expected diff output to include direct-child entries, got %q", output)
		}
		if strings.Contains(output, "/customers/nested/gamma/name") {
			t.Fatalf("expected non-recursive diff to exclude nested resources, got %q", output)
		}
	})

	t.Run("fallback_to_single_resource_when_collection_list_is_empty", func(t *testing.T) {
		t.Parallel()

		const idPath = "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c"

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/admin/realms/master/clients/account"},
			},
			getLocalValues: map[string]resource.Value{
				idPath: map[string]any{"id": "f88c68f3-3253-49f9-94a9-fe7553d33b5c", "clientId": "account"},
			},
			diffValues: map[string][]resource.DiffEntry{
				idPath: {
					{Path: idPath + "/name", Operation: "replace"},
				},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "diff", idPath)
		if err != nil {
			t.Fatalf("unexpected diff fallback error: %v", err)
		}
		if len(orchestrator.diffCalls) != 1 || orchestrator.diffCalls[0] != idPath {
			t.Fatalf("expected diff fallback single target %q, got %#v", idPath, orchestrator.diffCalls)
		}
		if !strings.Contains(output, idPath+"/name") {
			t.Fatalf("expected diff fallback output for %q, got %q", idPath, output)
		}
	})

	t.Run("fails_when_no_local_resources_match", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/acme"},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, "", "resource", "diff", "/orders")
		assertTypedCategory(t, err, faults.NotFoundError)
	})

	t.Run("text_output_uses_relative_dot_path_and_local_remote_values", func(t *testing.T) {
		t.Parallel()

		const targetPath = "/admin/realms/payments"

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: targetPath},
			},
			diffValues: map[string][]resource.DiffEntry{
				targetPath: {
					{
						Path:      "/admin/realms/payments/attributes/clientOfflineSessionIdleTimeout",
						Operation: "add",
						Local:     nil,
						Remote:    "0",
					},
					{
						Path:      "/admin/realms/payments/displayName",
						Operation: "replace",
						Local:     "Payments Realm",
						Remote:    "Payments Realm 2",
					},
				},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "diff", targetPath, "--output", "text")
		if err != nil {
			t.Fatalf("unexpected diff text output error: %v", err)
		}

		expectedOutput := strings.Join([]string{
			".attributes.clientOfflineSessionIdleTimeout [Local=null] => [Remote=\"0\"]",
			".displayName [Local=\"Payments Realm\"] => [Remote=\"Payments Realm 2\"]",
		}, "\n") + "\n"
		if output != expectedOutput {
			t.Fatalf("expected text output %q, got %q", expectedOutput, output)
		}
	})
}

func TestResourceCollectionMutationsFallbackToSingleResourceLookupWhenListIsEmpty(t *testing.T) {
	t.Parallel()

	const idPath = "/admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c"

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/admin/realms/master/clients/account"},
		},
		getLocalValues: map[string]resource.Value{
			idPath: map[string]any{"id": "f88c68f3-3253-49f9-94a9-fe7553d33b5c", "clientId": "account"},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	_, err := executeForTest(deps, "", "resource", "apply", idPath)
	if err != nil {
		t.Fatalf("unexpected apply fallback error: %v", err)
	}
	if len(orchestrator.applyCalls) != 1 || orchestrator.applyCalls[0] != idPath {
		t.Fatalf("expected apply to execute fallback single target %q, got %#v", idPath, orchestrator.applyCalls)
	}

	orchestrator.updateCalls = nil
	orchestrator.getLocalCalls = nil
	_, err = executeForTest(deps, "", "resource", "update", idPath)
	if err != nil {
		t.Fatalf("unexpected update fallback error: %v", err)
	}
	if len(orchestrator.updateCalls) != 1 || orchestrator.updateCalls[0].logicalPath != idPath {
		t.Fatalf("expected update to execute fallback single target %q, got %#v", idPath, orchestrator.updateCalls)
	}
	if len(orchestrator.getLocalCalls) == 0 || orchestrator.getLocalCalls[0] != idPath {
		t.Fatalf("expected getLocal fallback lookup for %q, got %#v", idPath, orchestrator.getLocalCalls)
	}
}

func TestResourceCollectionMutationsFailWhenNoLocalResourcesMatch(t *testing.T) {
	t.Parallel()

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		localList: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	_, err := executeForTest(deps, "", "resource", "apply", "/orders")
	assertTypedCategory(t, err, faults.NotFoundError)

	_, err = executeForTest(deps, "", "resource", "create", "/orders")
	assertTypedCategory(t, err, faults.NotFoundError)

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
	if strings.Contains(output, "--output=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
	if strings.Contains(output, "--context=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
	if strings.Contains(output, "--path=") {
		t.Fatalf("expected bash completion script to avoid duplicated equals flag suggestions, got %q", output)
	}
}

func TestPathCompletionResourceGetPrefersRemoteAndOpenAPI(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/customers/local"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/customers/remote"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/customers/{id}": map[string]any{},
			"/health":         map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/customers")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/customers/") {
		t.Fatalf("expected next-level collection completion, got %q", output)
	}
	if strings.Contains(output, "/customers/remote") || strings.Contains(output, "/customers/local") {
		t.Fatalf("expected get completion to collapse to next-level candidates, got %q", output)
	}
	if len(orchestrator.listLocalCalls) > 0 {
		t.Fatalf("expected get completion to skip local list when remote suggestions are available, calls=%#v", orchestrator.listLocalCalls)
	}
	if !strings.Contains(output, ":6") {
		t.Fatalf("expected no-file+no-space completion directive, got %q", output)
	}
}

func TestPathCompletionExpandsOpenAPITemplatesFromRemoteCollectionItems(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/master/clients/local-app"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/prod"},
		{LogicalPath: "/admin/realms/master/clients/remote-app"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/master/clients/remote-app") {
		t.Fatalf("expected remote collection item completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/clients/local-app") {
		t.Fatalf("expected get completion to avoid local fallback when remote collection items are available, got %q", output)
	}
	if !containsString(orchestrator.listRemoteCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected completion to consult remote collection path, calls=%#v", orchestrator.listRemoteCalls)
	}
	if containsString(orchestrator.listLocalCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected completion to skip local collection fallback when remote items are available, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionFallsBackToRepositoryWhenRemoteUnavailable(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/master/clients/local-app"},
	}
	orchestrator.listRemoteErr = errors.New("remote unavailable")
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/clients/local-app") {
		t.Fatalf("expected repository fallback completion item, got %q", output)
	}
	if !containsString(orchestrator.listLocalCalls, "/admin/realms/master/clients") {
		t.Fatalf("expected fallback completion to consult local collection path, calls=%#v", orchestrator.listLocalCalls)
	}
}

func TestPathCompletionResourceApplyPrefersRepository(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.localList = []resource.Resource{
		{LogicalPath: "/customers/local"},
	}
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/customers/remote"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "apply", "/customers")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/customers/") {
		t.Fatalf("expected next-level repository collection completion, got %q", output)
	}
	if strings.Contains(output, "/customers/remote") || strings.Contains(output, "/customers/local") {
		t.Fatalf("expected apply completion to collapse to next-level candidates, got %q", output)
	}
	if len(orchestrator.listRemoteCalls) > 0 {
		t.Fatalf("expected apply completion to skip remote list when local suggestions are available, calls=%#v", orchestrator.listRemoteCalls)
	}
}

func TestPathCompletionUsesAliasAttributeForCollectionItems(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{
			LogicalPath:    "/admin/realms/master/clients/f88c68f3",
			CollectionPath: "/admin/realms/master/clients",
			Metadata: metadatadomain.ResourceMetadata{
				AliasFromAttribute: "clientId",
				IDFromAttribute:    "id",
			},
			Payload: map[string]any{
				"id":       "f88c68f3",
				"clientId": "account",
			},
		},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/clients/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/master/clients/account") {
		t.Fatalf("expected alias-based completion item, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/clients/f88c68f3") {
		t.Fatalf("expected completion to hide id-based segment when alias is available, got %q", output)
	}
}

func TestPathCompletionRendersCollectionsWithTrailingSlash(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/clients/remote-app"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/adm")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/\n") {
		t.Fatalf("expected collection prefix with trailing slash, got %q", output)
	}
	if strings.Contains(output, "/admin\n") {
		t.Fatalf("expected collection prefix to render with trailing slash only, got %q", output)
	}
}

func TestPathCompletionCompletesScopedPrefixToCanonicalPath(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
		{LogicalPath: "/admin/realms/prod"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/rea")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/\n") {
		t.Fatalf("expected scoped completion to include canonical /admin/realms/, got %q", output)
	}
	if strings.Contains(output, "/realms\n") {
		t.Fatalf("expected completion to avoid fragment-only candidate output, got %q", output)
	}
}

func TestPathCompletionNoDescRemainsPrefixCompatibleForBash(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__completeNoDesc", "resource", "get", "/admin/rea")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/") {
		t.Fatalf("expected __completeNoDesc to emit prefix-compatible canonical candidates, got %q", output)
	}
	if strings.Contains(output, "\n/realms\n") {
		t.Fatalf("expected __completeNoDesc to avoid fragment-only candidates, got %q", output)
	}
}

func TestPathCompletionShowsOnlyNextLevelForCollectionPrefix(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/alpha/clients/app-a"},
		{LogicalPath: "/admin/realms/beta/roles/viewer"},
		{LogicalPath: "/admin/realms/gamma"},
	}
	orchestrator.openAPISpec = map[string]any{
		"paths": map[string]any{
			"/admin/realms/{realm}/clients/{clientId}": map[string]any{},
		},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/alpha/\n") {
		t.Fatalf("expected alpha realm next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/beta/\n") {
		t.Fatalf("expected beta realm next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/gamma\n") {
		t.Fatalf("expected gamma realm completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/alpha/clients") {
		t.Fatalf("expected collection prefix completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/beta/roles") {
		t.Fatalf("expected collection prefix completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "{realm}") {
		t.Fatalf("expected completion to suppress templated placeholders, got %q", output)
	}
}

func TestPathCompletionShowsOnlyNextLevelForNestedCollectionPrefix(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/master/aaaa/resource-a"},
		{LogicalPath: "/admin/realms/master/bbbb"},
		{LogicalPath: "/admin/realms/master/cccc/deeper/resource-c"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/master/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}

	if !strings.Contains(output, "/admin/realms/master/aaaa/\n") {
		t.Fatalf("expected aaaa next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/bbbb\n") {
		t.Fatalf("expected bbbb next-level completion, got %q", output)
	}
	if !strings.Contains(output, "/admin/realms/master/cccc/\n") {
		t.Fatalf("expected cccc next-level completion, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/aaaa/resource-a") {
		t.Fatalf("expected nested collection completion to omit deep descendants, got %q", output)
	}
	if strings.Contains(output, "/admin/realms/master/cccc/deeper") {
		t.Fatalf("expected nested collection completion to omit deep descendants, got %q", output)
	}
}

func TestPathCompletionScopedQueriesAvoidRootRecursiveFallbackWhenScopedMatchesExist(t *testing.T) {
	t.Parallel()

	deps := testDeps()
	orchestrator := deps.Orchestrator.(*testOrchestrator)
	orchestrator.remoteList = []resource.Resource{
		{LogicalPath: "/admin/realms/alpha"},
		{LogicalPath: "/admin/realms/beta"},
	}

	output, err := executeForTest(deps, "", "__complete", "resource", "get", "/admin/realms/")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if !strings.Contains(output, "/admin/realms/alpha") {
		t.Fatalf("expected scoped completion output, got %q", output)
	}
	if !containsListCall(orchestrator.listRemoteDetail, "/admin/realms", false) {
		t.Fatalf("expected scoped non-recursive query for /admin/realms, calls=%#v", orchestrator.listRemoteDetail)
	}
	if containsListCall(orchestrator.listRemoteDetail, "/", true) {
		t.Fatalf("expected scoped completion to avoid root-recursive remote fallback when scoped candidates exist, calls=%#v", orchestrator.listRemoteDetail)
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

func TestPathCompletionAvailableForAllPathAwareCommands(t *testing.T) {
	t.Parallel()

	command := NewRootCommand(testDeps())
	pathAwareCommands := [][]string{
		{"resource", "get"},
		{"resource", "save"},
		{"resource", "apply"},
		{"resource", "create"},
		{"resource", "update"},
		{"resource", "delete"},
		{"resource", "diff"},
		{"resource", "list"},
		{"resource", "explain"},
		{"resource", "template"},
		{"metadata", "get"},
		{"metadata", "set"},
		{"metadata", "unset"},
		{"metadata", "resolve"},
		{"metadata", "render"},
		{"metadata", "infer"},
		{"secret", "get"},
		{"secret", "detect"},
		{"ad-hoc", "get"},
		{"ad-hoc", "head"},
		{"ad-hoc", "options"},
		{"ad-hoc", "post"},
		{"ad-hoc", "put"},
		{"ad-hoc", "patch"},
		{"ad-hoc", "delete"},
		{"ad-hoc", "trace"},
		{"ad-hoc", "connect"},
	}

	for _, commandPath := range pathAwareCommands {
		commandPath := append([]string{}, commandPath...)
		t.Run(joinPath(commandPath), func(t *testing.T) {
			target := commandByPath(command, commandPath...)
			if target == nil {
				t.Fatalf("expected command path %q to exist", joinPath(commandPath))
			}
			if target.Flags().Lookup("path") == nil {
				t.Fatalf("expected command %q to declare --path", joinPath(commandPath))
			}
			if _, found := target.GetFlagCompletionFunc("path"); !found {
				t.Fatalf("expected command %q to register --path completion", joinPath(commandPath))
			}
			if target.ValidArgsFunction == nil {
				t.Fatalf("expected command %q to register positional path completion", joinPath(commandPath))
			}
		})
	}
}

func TestPathCompletionResourceSourceFlagsSwitchCompletionTarget(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		args            []string
		expectLocal     bool
		expectRemote    bool
		expectedSnippet string
	}{
		{
			name:            "get_default_prefers_remote",
			args:            []string{"resource", "get", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_repository_prefers_local",
			args:            []string{"resource", "get", "--repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_remote_server_prefers_remote",
			args:            []string{"resource", "get", "--remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "get_repository_path_flag_uses_local",
			args:            []string{"resource", "get", "--repository", "--path", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "list_repository_prefers_local",
			args:            []string{"resource", "list", "--repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "list_remote_server_prefers_remote",
			args:            []string{"resource", "list", "--remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_repository_prefers_local",
			args:            []string{"resource", "delete", "--force", "--repository", "/customers"},
			expectLocal:     true,
			expectRemote:    false,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_remote_server_prefers_remote",
			args:            []string{"resource", "delete", "--force", "--remote-server", "/customers"},
			expectLocal:     false,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
		{
			name:            "delete_both_queries_both_sources",
			args:            []string{"resource", "delete", "--force", "--both", "/customers"},
			expectLocal:     true,
			expectRemote:    true,
			expectedSnippet: "/customers/",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			deps := testDeps()
			orchestrator := deps.Orchestrator.(*testOrchestrator)
			orchestrator.localList = []resource.Resource{
				{LogicalPath: "/customers/local"},
			}
			orchestrator.remoteList = []resource.Resource{
				{LogicalPath: "/customers/remote"},
			}

			completeArgs := append([]string{"__complete"}, testCase.args...)
			output, err := executeForTest(deps, "", completeArgs...)
			if err != nil {
				t.Fatalf("unexpected completion error: %v", err)
			}
			if !strings.Contains(output, testCase.expectedSnippet) {
				t.Fatalf("expected completion output %q, got %q", testCase.expectedSnippet, output)
			}
			if !strings.Contains(output, ":6") {
				t.Fatalf("expected path completion directive :6, got %q", output)
			}

			localCalled := len(orchestrator.listLocalCalls) > 0
			remoteCalled := len(orchestrator.listRemoteCalls) > 0

			if localCalled != testCase.expectLocal {
				t.Fatalf("expected local completion queries=%t, got %t (calls=%#v)", testCase.expectLocal, localCalled, orchestrator.listLocalDetail)
			}
			if remoteCalled != testCase.expectRemote {
				t.Fatalf("expected remote completion queries=%t, got %t (calls=%#v)", testCase.expectRemote, remoteCalled, orchestrator.listRemoteDetail)
			}
		})
	}
}

func TestPathCompletionAdHocPrefersRemoteWithRepositoryFallback(t *testing.T) {
	t.Parallel()

	t.Run("remote_first", func(t *testing.T) {
		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.localList = []resource.Resource{
			{LogicalPath: "/admin/local-only"},
		}
		orchestrator.remoteList = []resource.Resource{
			{LogicalPath: "/admin/remote-only"},
		}

		output, err := executeForTest(deps, "", "__complete", "ad-hoc", "get", "/adm")
		if err != nil {
			t.Fatalf("unexpected completion error: %v", err)
		}
		if !strings.Contains(output, "/admin/") {
			t.Fatalf("expected ad-hoc completion output, got %q", output)
		}
		if len(orchestrator.listRemoteCalls) == 0 {
			t.Fatalf("expected ad-hoc completion to query remote source first")
		}
		if len(orchestrator.listLocalCalls) != 0 {
			t.Fatalf("expected ad-hoc completion to skip local fallback when remote candidates exist, calls=%#v", orchestrator.listLocalDetail)
		}
	})

	t.Run("fallback_to_local_when_remote_fails", func(t *testing.T) {
		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.localList = []resource.Resource{
			{LogicalPath: "/admin/local-only"},
		}
		orchestrator.listRemoteErr = errors.New("remote unavailable")

		output, err := executeForTest(deps, "", "__complete", "ad-hoc", "get", "/adm")
		if err != nil {
			t.Fatalf("unexpected completion error: %v", err)
		}
		if !strings.Contains(output, "/admin/") {
			t.Fatalf("expected ad-hoc completion output from local fallback, got %q", output)
		}
		if len(orchestrator.listRemoteCalls) == 0 {
			t.Fatalf("expected ad-hoc completion to attempt remote source first")
		}
		if len(orchestrator.listLocalCalls) == 0 {
			t.Fatalf("expected ad-hoc completion to fallback to local source when remote fails")
		}
	})
}

func TestPathCompletionMetadataAndSecretPreferRepositoryPaths(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		args []string
	}{
		{name: "metadata_get", args: []string{"metadata", "get", "/customers"}},
		{name: "secret_get", args: []string{"secret", "get", "/customers"}},
		{name: "secret_detect", args: []string{"secret", "detect", "/customers"}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			deps := testDeps()
			orchestrator := deps.Orchestrator.(*testOrchestrator)
			orchestrator.localList = []resource.Resource{
				{LogicalPath: "/customers/local"},
			}
			orchestrator.remoteList = []resource.Resource{
				{LogicalPath: "/customers/remote"},
			}

			completeArgs := append([]string{"__complete"}, testCase.args...)
			output, err := executeForTest(deps, "", completeArgs...)
			if err != nil {
				t.Fatalf("unexpected completion error: %v", err)
			}
			if !strings.Contains(output, "/customers/") {
				t.Fatalf("expected completion output, got %q", output)
			}
			if len(orchestrator.listLocalCalls) == 0 {
				t.Fatalf("expected completion to query local repository source")
			}
			if len(orchestrator.listRemoteCalls) != 0 {
				t.Fatalf("expected completion to skip remote fallback when repository candidates exist, calls=%#v", orchestrator.listRemoteDetail)
			}
		})
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
		{name: "ad-hoc", args: []string{"ad-hoc"}, expectedSnippet: "Execute ad-hoc HTTP requests against resource server API"},
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
	if !strings.Contains(output, "--force") {
		t.Fatalf("expected --force in resource save help output, got %q", output)
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

func commandByPath(root *cobra.Command, path ...string) *cobra.Command {
	command := root
	for _, name := range path {
		found := false
		for _, child := range command.Commands() {
			if child.Name() != name {
				continue
			}
			command = child
			found = true
			break
		}
		if !found {
			return nil
		}
	}
	return command
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
		&testOrchestrator{
			metadataService: metadataService,
		},
		metadataService,
	)
}

func testDepsWith(orchestrator *testOrchestrator, metadataService *testMetadata) Dependencies {
	secretProvider := newTestSecretProvider()
	repositoryService := &testRepository{}

	return Dependencies{
		Orchestrator: orchestrator,
		Contexts:     &testContextService{},
		Repository:   repositoryService,
		Metadata:     metadataService,
		Secrets:      secretProvider,
	}
}

func newResourceSaveDeps(orchestrator *testOrchestrator, metadataService *testMetadata) Dependencies {
	deps := testDepsWith(orchestrator, metadataService)
	deps.Repository = &resourceSaveTestRepository{}
	return deps
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

	repositoryConfig := config.Repository{
		ResourceFormat: format,
		Filesystem:     &config.FilesystemRepository{BaseDir: "/tmp/repo"},
	}
	if name == "git" || name == "git-no-remote" {
		gitRepo := &config.GitRepository{
			Local: config.GitLocal{
				BaseDir: "/tmp/repo",
			},
		}
		if name == "git" {
			gitRepo.Remote = &config.GitRemote{
				URL: "https://example.invalid/repo.git",
			}
		}
		repositoryConfig = config.Repository{
			ResourceFormat: format,
			Git:            gitRepo,
		}
	}

	return config.Context{
		Name:       name,
		Repository: repositoryConfig,
	}, nil
}
func (s *testContextService) Validate(context.Context, config.Context) error { return nil }

type testOrchestrator struct {
	metadataService  *testMetadata
	saveCalls        []savedResource
	deleteCalls      []deleteCall
	saveErr          error
	getRemoteValue   resource.Value
	getRemoteValues  map[string]resource.Value
	getRemoteErr     error
	getRemoteCalls   []string
	adHocCalls       []adHocCall
	adHocErr         error
	getLocalCalls    []string
	listLocalCalls   []string
	listLocalDetail  []listCall
	listRemoteCalls  []string
	listRemoteDetail []listCall
	listRemoteErr    error
	applyCalls       []string
	createCalls      []savedResource
	updateCalls      []savedResource
	diffCalls        []string
	diffValues       map[string][]resource.DiffEntry
	diffErr          error
	getLocalValues   map[string]resource.Value
	localList        []resource.Resource
	remoteList       []resource.Resource
	openAPISpec      resource.Value
}

type savedResource struct {
	logicalPath string
	value       resource.Value
}

type deleteCall struct {
	logicalPath string
	recursive   bool
}

type listCall struct {
	logicalPath string
	recursive   bool
}

type adHocCall struct {
	method string
	path   string
	body   resource.Value
}

func (r *testOrchestrator) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath, "source": "get"}, nil
}
func (r *testOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Value, error) {
	r.getLocalCalls = append(r.getLocalCalls, logicalPath)
	if r.getLocalValues != nil {
		if value, ok := r.getLocalValues[logicalPath]; ok {
			return value, nil
		}
	}
	return map[string]any{"path": logicalPath, "source": "local"}, nil
}
func (r *testOrchestrator) GetRemote(_ context.Context, logicalPath string) (resource.Value, error) {
	r.getRemoteCalls = append(r.getRemoteCalls, logicalPath)
	if r.getRemoteErr != nil {
		return nil, r.getRemoteErr
	}
	if r.getRemoteValues != nil {
		if value, ok := r.getRemoteValues[logicalPath]; ok {
			return value, nil
		}
		return nil, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
	}
	if r.getRemoteValue != nil {
		return r.getRemoteValue, nil
	}
	return map[string]any{"path": logicalPath, "source": "remote"}, nil
}
func (r *testOrchestrator) AdHoc(_ context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error) {
	r.adHocCalls = append(r.adHocCalls, adHocCall{
		method: method,
		path:   endpointPath,
		body:   body,
	})
	if r.adHocErr != nil {
		return nil, r.adHocErr
	}
	return map[string]any{
		"method": method,
		"path":   endpointPath,
		"body":   body,
	}, nil
}
func (r *testOrchestrator) GetOpenAPISpec(_ context.Context) (resource.Value, error) {
	return r.openAPISpec, nil
}
func (r *testOrchestrator) Save(_ context.Context, logicalPath string, value resource.Value) error {
	r.saveCalls = append(r.saveCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return r.saveErr
}
func (r *testOrchestrator) Apply(_ context.Context, logicalPath string) (resource.Resource, error) {
	r.applyCalls = append(r.applyCalls, logicalPath)
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testOrchestrator) Create(_ context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	r.createCalls = append(r.createCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testOrchestrator) Update(_ context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	r.updateCalls = append(r.updateCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testOrchestrator) Delete(_ context.Context, logicalPath string, policy orchestrator.DeletePolicy) error {
	r.deleteCalls = append(r.deleteCalls, deleteCall{
		logicalPath: logicalPath,
		recursive:   policy.Recursive,
	})
	return nil
}
func (r *testOrchestrator) ListLocal(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	r.listLocalCalls = append(r.listLocalCalls, logicalPath)
	r.listLocalDetail = append(r.listLocalDetail, listCall{
		logicalPath: logicalPath,
		recursive:   policy.Recursive,
	})
	if len(r.localList) > 0 {
		items := make([]resource.Resource, len(r.localList))
		copy(items, r.localList)
		filtered := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			if policy.Recursive && isPathOrDescendant(logicalPath, item.LogicalPath) {
				filtered = append(filtered, item)
				continue
			}
			if !policy.Recursive && isDirectChildPath(logicalPath, item.LogicalPath) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
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
func (r *testOrchestrator) ListRemote(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	r.listRemoteCalls = append(r.listRemoteCalls, logicalPath)
	r.listRemoteDetail = append(r.listRemoteDetail, listCall{
		logicalPath: logicalPath,
		recursive:   policy.Recursive,
	})
	if r.listRemoteErr != nil {
		return nil, r.listRemoteErr
	}
	if len(r.remoteList) > 0 {
		items := make([]resource.Resource, len(r.remoteList))
		copy(items, r.remoteList)
		filtered := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			if policy.Recursive && isPathOrDescendant(logicalPath, item.LogicalPath) {
				filtered = append(filtered, item)
				continue
			}
			if !policy.Recursive && isDirectChildPath(logicalPath, item.LogicalPath) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
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
func (r *testOrchestrator) Explain(_ context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	return []resource.DiffEntry{{Path: logicalPath, Operation: "noop"}}, nil
}
func (r *testOrchestrator) Diff(_ context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	r.diffCalls = append(r.diffCalls, logicalPath)
	if r.diffErr != nil {
		return nil, r.diffErr
	}
	if r.diffValues != nil {
		if value, ok := r.diffValues[logicalPath]; ok {
			items := make([]resource.DiffEntry, len(value))
			copy(items, value)
			return items, nil
		}
	}
	return []resource.DiffEntry{{Path: logicalPath, Operation: "noop"}}, nil
}
func (r *testOrchestrator) Template(_ context.Context, _ string, value resource.Value) (resource.Value, error) {
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

func isPathOrDescendant(basePath string, candidatePath string) bool {
	base := path.Clean(basePath)
	candidate := path.Clean(candidatePath)
	if base == "/" {
		return strings.HasPrefix(candidate, "/")
	}
	if base == candidate {
		return true
	}
	basePrefix := strings.TrimSuffix(base, "/")
	return strings.HasPrefix(candidate, basePrefix+"/")
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsListCall(items []listCall, logicalPath string, recursive bool) bool {
	for _, item := range items {
		if item.logicalPath == logicalPath && item.recursive == recursive {
			return true
		}
	}
	return false
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
	pushCalls   int
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
func (r *testRepository) Push(context.Context, repository.PushPolicy) error {
	r.pushCalls++
	return nil
}
func (r *testRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
}

type resourceSaveTestRepository struct {
	values map[string]resource.Value
}

func (r *resourceSaveTestRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	if r.values == nil {
		r.values = map[string]resource.Value{}
	}
	r.values[logicalPath] = value
	return nil
}

func (r *resourceSaveTestRepository) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	if r.values != nil {
		if value, found := r.values[logicalPath]; found {
			return value, nil
		}
	}
	return nil, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
}

func (r *resourceSaveTestRepository) Delete(_ context.Context, _ string, _ repository.DeletePolicy) error {
	return nil
}

func (r *resourceSaveTestRepository) List(_ context.Context, _ string, _ repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}

func (r *resourceSaveTestRepository) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (r *resourceSaveTestRepository) Move(context.Context, string, string) error          { return nil }
func (r *resourceSaveTestRepository) Init(context.Context) error                          { return nil }
func (r *resourceSaveTestRepository) Refresh(context.Context) error                       { return nil }
func (r *resourceSaveTestRepository) Reset(context.Context, repository.ResetPolicy) error { return nil }
func (r *resourceSaveTestRepository) Check(context.Context) error                         { return nil }
func (r *resourceSaveTestRepository) Push(context.Context, repository.PushPolicy) error   { return nil }
func (r *resourceSaveTestRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
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
