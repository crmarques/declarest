package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	fsmetadata "github.com/crmarques/declarest/internal/providers/metadata/fs"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	serverdomain "github.com/crmarques/declarest/server"
)

func TestRequiredCommandPathsRegistered(t *testing.T) {
	t.Parallel()

	requiredPaths := []string{
		"config",
		"config print-template",
		"config add",
		"config edit",
		"config use",
		"config current",
		"config check",
		"config resolve",
		"resource",
		"resource get",
		"resource delete",
		"resource edit",
		"resource copy",
		"resource-server",
		"resource-server get",
		"resource-server get base-url",
		"resource-server get token-url",
		"resource-server get access-token",
		"resource-server check",
		"metadata",
		"metadata resolve",
		"metadata render",
		"repo",
		"repo clean",
		"repo commit",
		"repo status",
		"repo tree",
		"repo history",
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
	if !strings.Contains(output, "\n  resource ") {
		t.Fatalf("expected resource command to be present in root help, got %q", output)
	}
}

func TestMissingPositionalParameterValidationPrintsUsage(t *testing.T) {
	t.Parallel()

	t.Run("missing positionals prints usage", func(t *testing.T) {
		t.Parallel()

		_, stderr, err := executeForTestWithStreams(testDeps(), "", "resource", "copy")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "path is required") {
			t.Fatalf("expected missing path validation error, got %v", err)
		}
		if !strings.Contains(stderr, "Usage:") {
			t.Fatalf("expected usage output on stderr, got %q", stderr)
		}
		if !strings.Contains(stderr, "declarest resource copy [path] [target-path]") {
			t.Fatalf("expected resource copy usage line, got %q", stderr)
		}
	})

	t.Run("validation with provided args does not print usage", func(t *testing.T) {
		t.Parallel()

		_, stderr, err := executeForTestWithStreams(testDeps(), "", "resource", "get", "/customers/a", "--path", "/customers/b")
		assertTypedCategory(t, err, faults.ValidationError)
		if strings.Contains(stderr, "Usage:") {
			t.Fatalf("did not expect usage output for mismatch validation, got %q", stderr)
		}
	})
}

func TestResourceServerGet(t *testing.T) {
	t.Parallel()

	t.Run("prints_base_url", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource-server", "get", "base-url")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "https://api.example.invalid\n" {
			t.Fatalf("expected base-url output, got %q", output)
		}
	})

	t.Run("prints_token_url", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource-server", "get", "token-url")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "https://auth.example.invalid/oauth/token\n" {
			t.Fatalf("expected token-url output, got %q", output)
		}
	})

	t.Run("token_url_fails_when_oauth2_not_configured", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "--context", "bearer", "resource-server", "get", "token-url")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "oauth2") {
			t.Fatalf("expected oauth2 validation error, got %v", err)
		}
	})

	t.Run("prints_access_token", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		deps.ResourceServer = &testResourceServer{accessToken: "oauth-access-token"}

		output, err := executeForTest(deps, "", "resource-server", "get", "access-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "oauth-access-token\n" {
			t.Fatalf("expected token output, got %q", output)
		}
	})

	t.Run("fails_when_oauth2_not_configured", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		deps.ResourceServer = &testResourceServer{
			tokenErr: faults.NewTypedError(faults.ValidationError, "resource-server.http.auth.oauth2 is not configured", nil),
		}

		_, err := executeForTest(deps, "", "resource-server", "get", "access-token")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "oauth2") {
			t.Fatalf("expected oauth2 validation error, got %v", err)
		}
	})
}

func TestResourceServerCheck(t *testing.T) {
	t.Parallel()

	t.Run("success_probe", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestratorService := deps.Orchestrator.(*testOrchestrator)

		output, err := executeForTest(deps, "", "resource-server", "check")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "resource-server check: OK") {
			t.Fatalf("expected success output, got %q", output)
		}
		if !containsListCall(orchestratorService.listRemoteDetail, "/", false) {
			t.Fatalf("expected root remote probe, got %#v", orchestratorService.listRemoteDetail)
		}
	})

	t.Run("warn_categories_return_ok", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		deps.Orchestrator.(*testOrchestrator).listRemoteErr = faults.NewTypedError(
			faults.NotFoundError,
			"collection not found",
			nil,
		)

		output, err := executeForTest(deps, "", "resource-server", "check")
		if err != nil {
			t.Fatalf("expected warning probe to return success, got %v", err)
		}
		if !strings.Contains(output, "returned NotFoundError") {
			t.Fatalf("expected warning category in output, got %q", output)
		}
		if !strings.Contains(output, "collection not found") {
			t.Fatalf("expected underlying error detail in output, got %q", output)
		}
	})

	t.Run("auth_error_fails", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		deps.Orchestrator.(*testOrchestrator).listRemoteErr = faults.NewTypedError(
			faults.AuthError,
			"resource server auth failed",
			nil,
		)

		_, err := executeForTest(deps, "", "resource-server", "check")
		assertTypedCategory(t, err, faults.AuthError)
	})
}

func TestOutputPolicyValidation(t *testing.T) {
	t.Parallel()

	t.Run("resource_server_plain_text_commands_reject_structured_output", func(t *testing.T) {
		t.Parallel()
		_, err := executeForTest(testDeps(), "", "--output", "json", "resource-server", "get", "base-url")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("repo_tree_rejects_structured_output", func(t *testing.T) {
		t.Parallel()
		_, err := executeForTest(testDeps(), "", "--output", "json", "repo", "tree")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("config_show_rejects_json_output", func(t *testing.T) {
		t.Parallel()
		_, err := executeForTest(testDeps(), "", "--output", "json", "config", "show")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("config_show_allows_yaml_output", func(t *testing.T) {
		t.Parallel()
		output, err := executeForTest(testDeps(), "", "--context", "dev", "--output", "yaml", "config", "show")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "resource-server:") {
			t.Fatalf("expected yaml context output, got %q", output)
		}
	})
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
	if !strings.Contains(debugOutput, `debug: root flags context="" output="auto" verbose=false no_status=false no_color=false command="declarest resource get"`) {
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
	if !strings.Contains(debugOutput, `debug: metadata fs resolve lookup selector="/admin/realms" kind="collection"`) {
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

	t.Run("invalid_non_absolute_path_fails_fast", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		_, err := executeForTest(deps, "", "resource", "get", "adminrealmspublico-bruser-registryADmappers")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "absolute") {
			t.Fatalf("expected absolute-path validation error, got %v", err)
		}
		if len(orchestrator.getRemoteCalls) > 0 || len(orchestrator.getLocalCalls) > 0 {
			t.Fatalf(
				"expected invalid path to fail before source requests, remote_calls=%#v local_calls=%#v",
				orchestrator.getRemoteCalls,
				orchestrator.getLocalCalls,
			)
		}
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

	t.Run("source_repository_uses_repository", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--source", "repository")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"local\"") {
			t.Fatalf("expected repository source output, got %q", output)
		}
	})

	t.Run("http_method_override_requires_remote_source", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--source", "repository", "--http-method", "POST")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--http-method") {
			t.Fatalf("expected http-method/source validation error, got %v", err)
		}
	})

	t.Run("source_remote_server_uses_remote", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--source", "remote-server")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"remote\"") {
			t.Fatalf("expected remote source output, got %q", output)
		}
	})

	t.Run("remote_collection_marker_uses_list_remote", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.remoteList = []resource.Resource{
			{
				LogicalPath: "/admin/realms/publico-br/user-registry/AD/mappers/alpha",
				Payload: map[string]any{
					"id":   "mapper-a",
					"name": "alpha",
				},
			},
			{
				LogicalPath: "/admin/realms/publico-br/user-registry/AD/mappers/beta",
				Payload: map[string]any{
					"id":   "mapper-b",
					"name": "beta",
				},
			},
		}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/admin/realms/publico-br/user-registry/AD/mappers/",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(orchestrator.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote get calls for explicit collection target, got %#v", orchestrator.getRemoteCalls)
		}
		if !reflect.DeepEqual(
			orchestrator.listRemoteCalls,
			[]string{"/admin/realms/publico-br/user-registry/AD/mappers"},
		) {
			t.Fatalf("expected remote list call for normalized collection path, got %#v", orchestrator.listRemoteCalls)
		}
		if !strings.Contains(output, "\"name\": \"alpha\"") || !strings.Contains(output, "\"name\": \"beta\"") {
			t.Fatalf("expected collection payload output, got %q", output)
		}
	})

	t.Run("remote_collection_marker_falls_back_to_single_resource_on_invalid_list_shape", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.listRemoteErr = serverdomain.NewListPayloadShapeError(
			`list response object is ambiguous: expected an "items" array or a single array field, found array fields [enabledEventTypes, eventsListeners]`,
			nil,
		)
		orchestrator.getRemoteValue = map[string]any{
			"id":    "master",
			"realm": "master",
		}

		output, err := executeForTest(deps, "", "resource", "get", "/admin/realms/master/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master"}) {
			t.Fatalf("expected one remote collection list attempt, got %#v", orchestrator.listRemoteCalls)
		}
		if !reflect.DeepEqual(orchestrator.getRemoteCalls, []string{"/admin/realms/master"}) {
			t.Fatalf("expected one remote single-resource fallback attempt, got %#v", orchestrator.getRemoteCalls)
		}
		if !strings.Contains(output, `"realm": "master"`) {
			t.Fatalf("expected fallback single-resource output, got %q", output)
		}
	})

	t.Run("remote_not_found_without_collection_marker_renders_empty_collection_when_list_is_empty", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteErr = faults.NewTypedError(faults.NotFoundError, "resource not found", nil)
		orchestrator.remoteList = []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/clients/account",
				Payload: map[string]any{
					"id":       "account",
					"clientId": "account",
				},
			},
		}

		output, err := executeForTest(deps, "", "resource", "get", "/admin/realms/master/user-registry")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(orchestrator.getRemoteCalls, []string{"/admin/realms/master/user-registry"}) {
			t.Fatalf("expected one remote get call, got %#v", orchestrator.getRemoteCalls)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master/user-registry"}) {
			t.Fatalf("expected one remote list fallback call, got %#v", orchestrator.listRemoteCalls)
		}
		if strings.TrimSpace(output) != "[]" {
			t.Fatalf("expected empty collection output, got %q", output)
		}
	})

	t.Run("remote_not_found_without_collection_marker_keeps_not_found_when_list_has_items", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteErr = faults.NewTypedError(faults.NotFoundError, "resource not found", nil)
		orchestrator.remoteList = []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/user-registry/ldap-1",
				Payload: map[string]any{
					"id":   "ldap-id-1",
					"name": "ldap-1",
				},
			},
		}

		_, err := executeForTest(deps, "", "resource", "get", "/admin/realms/master/user-registry")
		assertTypedCategory(t, err, faults.NotFoundError)
		if !reflect.DeepEqual(orchestrator.getRemoteCalls, []string{"/admin/realms/master/user-registry"}) {
			t.Fatalf("expected one remote get call, got %#v", orchestrator.getRemoteCalls)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master/user-registry"}) {
			t.Fatalf("expected one remote list fallback call, got %#v", orchestrator.listRemoteCalls)
		}
	})

	t.Run("remote_not_found_without_collection_marker_renders_collection_when_metadata_declares_branch", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteErr = faults.NewTypedError(faults.NotFoundError, "resource not found", nil)
		orchestrator.remoteList = []resource.Resource{
			{
				LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha",
				Payload: map[string]any{
					"id":   "mapper-a",
					"name": "alpha",
				},
			},
			{
				LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/beta",
				Payload: map[string]any{
					"id":   "mapper-b",
					"name": "beta",
				},
			},
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.collectionChildren["/admin/realms/master/user-registry/AD PRD"] = []string{"mappers"}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/admin/realms/master/user-registry/AD PRD/mappers",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(orchestrator.getRemoteCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one remote get call, got %#v", orchestrator.getRemoteCalls)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one remote list fallback call, got %#v", orchestrator.listRemoteCalls)
		}
		if !strings.Contains(output, "\"name\": \"alpha\"") || !strings.Contains(output, "\"name\": \"beta\"") {
			t.Fatalf("expected collection payload output, got %q", output)
		}
	})

	t.Run("legacy_repository_and_remote_server_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("source_and_legacy_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--source", "repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("invalid_source_value_fails", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--source", "both")
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

	t.Run("show_metadata_flag_returns_rendered_metadata", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteValue = map[string]any{
			"id":    "acme",
			"realm": "master",
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
			Operations: map[string]metadatadomain.OperationSpec{
				string(metadatadomain.OperationGet): {
					Path: "/api/customers/acme",
				},
			},
		}

		output, err := executeForTest(
			deps,
			"",
			"resource",
			"get",
			"/customers/acme",
			"--remote-server",
			"--show-metadata",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("failed to decode metadata output: %v", err)
		}

		payload, ok := parsed["payload"].(map[string]any)
		if !ok || payload["id"] != "acme" {
			t.Fatalf("expected payload id acme, got %#v", payload)
		}

		metadataValue, ok := parsed["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("expected metadata section, got %#v", parsed["metadata"])
		}
		resourceInfo, ok := metadataValue["resourceInfo"].(map[string]any)
		if !ok {
			t.Fatalf("expected metadata.resourceInfo, got %#v", metadataValue["resourceInfo"])
		}
		if resourceInfo["collectionPath"] != "/customers" {
			t.Fatalf("expected collectionPath /customers, got %v", resourceInfo["collectionPath"])
		}

		operationInfo, ok := metadataValue["operationInfo"].(map[string]any)
		if !ok {
			t.Fatalf("expected operationInfo map, got %#v", metadataValue["operationInfo"])
		}
		getOp, ok := operationInfo["getResource"].(map[string]any)
		if !ok {
			t.Fatalf("expected getResource operation, got %#v", operationInfo["getResource"])
		}
		if getOp["path"] != "/api/customers/acme" {
			t.Fatalf("expected get path override, got %v", getOp["path"])
		}
		if _, hasAccept := getOp["accept"]; hasAccept {
			t.Fatalf("expected getResource.accept to be omitted from metadata output, got %#v", getOp["accept"])
		}
		assertOperationHTTPHeaderValue(t, getOp, "Accept", "application/json")
		createOp, ok := operationInfo["createResource"].(map[string]any)
		if !ok {
			t.Fatalf("expected createResource operation, got %#v", operationInfo["createResource"])
		}
		if createOp["path"] != "/customers" {
			t.Fatalf("expected create default path, got %v", createOp["path"])
		}
		if _, hasAccept := createOp["accept"]; hasAccept {
			t.Fatalf("expected createResource.accept to be omitted from metadata output, got %#v", createOp["accept"])
		}
		if _, hasContentType := createOp["contentType"]; hasContentType {
			t.Fatalf("expected createResource.contentType to be omitted from metadata output, got %#v", createOp["contentType"])
		}
		assertOperationHTTPHeaderValue(t, createOp, "Accept", "application/json")
		assertOperationHTTPHeaderValue(t, createOp, "Content-Type", "application/json")
	})

	t.Run("show_metadata_flag_uses_repository_resource_format_not_output_format", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.getRemoteValue = map[string]any{
			"id": "acme",
		}
		metadataService := deps.Metadata.(*testMetadata)
		metadataService.items["/customers/acme"] = metadatadomain.ResourceMetadata{
			IDFromAttribute: "id",
		}

		output, err := executeForTest(
			deps,
			"",
			"--context",
			"yaml",
			"-o",
			"json",
			"resource",
			"get",
			"/customers/acme",
			"--remote-server",
			"--show-metadata",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var parsed map[string]any
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("failed to decode metadata output: %v", err)
		}
		metadataValue, ok := parsed["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("expected metadata section, got %#v", parsed["metadata"])
		}
		operationInfo, ok := metadataValue["operationInfo"].(map[string]any)
		if !ok {
			t.Fatalf("expected operationInfo map, got %#v", metadataValue["operationInfo"])
		}
		getOp, ok := operationInfo["getResource"].(map[string]any)
		if !ok {
			t.Fatalf("expected getResource operation, got %#v", operationInfo["getResource"])
		}
		if _, hasAccept := getOp["accept"]; hasAccept {
			t.Fatalf("expected getResource.accept to be omitted from metadata output, got %#v", getOp["accept"])
		}
		assertOperationHTTPHeaderValue(t, getOp, "Accept", "application/yaml")
		createOp, ok := operationInfo["createResource"].(map[string]any)
		if !ok {
			t.Fatalf("expected createResource operation, got %#v", operationInfo["createResource"])
		}
		if _, hasContentType := createOp["contentType"]; hasContentType {
			t.Fatalf("expected createResource.contentType to be omitted from metadata output, got %#v", createOp["contentType"])
		}
		assertOperationHTTPHeaderValue(t, createOp, "Content-Type", "application/yaml")
	})
}

func TestResourceRequestMethodCommands(t *testing.T) {
	t.Parallel()

	t.Run("get_positional_path", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "request", "get", "/test")
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

	t.Run("get_not_found_does_not_run_cli_fallback", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		orchestrator.requestErr = faults.NewTypedError(faults.NotFoundError, "request path not found", nil)

		_, err := executeForTest(deps, "", "resource", "request", "get", "/admin/realms/master/clients/account")
		if err == nil {
			t.Fatal("expected error to be returned by CLI when orchestrator request returns not found")
		}
		if len(orchestrator.getRemoteCalls) != 0 {
			t.Fatalf("expected no CLI-level metadata-aware fallback read, got %#v", orchestrator.getRemoteCalls)
		}
	})

	t.Run("post_reads_stdin_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), `{"id":"a","name":"alpha"}`, "resource", "request", "post", "/items")
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

		output, err := executeForTest(testDeps(), "", "resource", "request", "put", "/items/a", "--payload", payloadPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected put output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("post_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "request", "post", "/items", "--payload", `{"id":"a","name":"gamma"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected post output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("put_reads_payload_flag_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "request", "put", "/items/a", "--payload", `{"id":"a","name":"delta"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected put output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("post_verbose_renders_response_body", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), `{"id":"a","name":"alpha"}`, "resource", "request", "post", "/items", "--verbose")
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
			"resource", "request", "post",
			"/items",
			"--payload",
			`{"id":"a","name":"gamma"}`,
			"--payload",
			payloadPath,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("payload_conflicts_with_stdin", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(
			testDeps(),
			`{"id":"a","name":"from-stdin"}`,
			"resource", "request", "post",
			"/items",
			"--payload",
			`{"id":"a","name":"from-flag"}`,
		)
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("delete_requires_force", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "request", "delete", "/items/a")
		assertTypedCategory(t, err, faults.ValidationError)
		if !strings.Contains(err.Error(), "--confirm-delete") {
			t.Fatalf("expected --confirm-delete validation message, got %v", err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "are you sure") {
			t.Fatalf("expected confirmation-style validation message, got %v", err)
		}
	})

	t.Run("delete_with_confirm_delete", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "request", "delete", "/items/a", "--confirm-delete")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected delete output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("delete_with_confirm_delete_verbose", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "request", "delete", "/items/a", "--confirm-delete", "--verbose")
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

		output, err := executeForTest(deps, "", "resource", "request", "delete", "/items", "--confirm-delete")
		if err != nil {
			t.Fatalf("unexpected collection delete error: %v", err)
		}
		if len(orchestrator.requestCalls) != 2 {
			t.Fatalf("expected 2 request delete calls, got %#v", orchestrator.requestCalls)
		}
		if orchestrator.requestCalls[0].path != "/items/a" || orchestrator.requestCalls[1].path != "/items/b" {
			t.Fatalf("expected direct-child delete paths [/items/a /items/b], got %#v", orchestrator.requestCalls)
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

		output, err := executeForTest(deps, "", "resource", "request", "delete", "/items", "--confirm-delete", "--recursive")
		if err != nil {
			t.Fatalf("unexpected recursive collection delete error: %v", err)
		}
		if len(orchestrator.requestCalls) != 3 {
			t.Fatalf("expected 3 request delete calls, got %#v", orchestrator.requestCalls)
		}
		if orchestrator.requestCalls[2].path != "/items/nested/c" {
			t.Fatalf("expected recursive delete to include nested path, got %#v", orchestrator.requestCalls)
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

		output, err := executeForTest(deps, "", "resource", "request", "delete", "/items", "--confirm-delete", "--recursive", "--verbose")
		if err != nil {
			t.Fatalf("unexpected recursive collection delete error: %v", err)
		}
		if !strings.Contains(output, "\"path\": \"/items/nested/c\"") {
			t.Fatalf("expected recursive delete output to include nested path with --verbose, got %q", output)
		}
	})

	t.Run("path_mismatch_fails_validation", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "request", "delete", "/a", "--path", "/b", "--confirm-delete")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("delete_legacy_force_alias_still_works", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "request", "delete", "/items/a", "--force")
		if err != nil {
			t.Fatalf("unexpected error with legacy --force alias: %v", err)
		}
	})
}

func TestResourceMutationExplicitPayloadInlineInputs(t *testing.T) {
	t.Parallel()

	t.Run("create_accepts_inline_json_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		output, err := executeForTest(
			deps,
			"",
			"resource", "create",
			"/customers/acme",
			"--payload", `{"id":"acme","name":"Acme"}`,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected create output to be empty without --verbose, got %q", output)
		}
		if len(orchestrator.createCalls) != 1 {
			t.Fatalf("expected one create call, got %d", len(orchestrator.createCalls))
		}
		body, ok := orchestrator.createCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", orchestrator.createCalls[0].value)
		}
		if body["name"] != "Acme" {
			t.Fatalf("expected inline payload name to be forwarded, got %#v", body)
		}
	})

	t.Run("apply_accepts_dotted_assignment_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		output, err := executeForTest(
			deps,
			"",
			"resource", "apply",
			"/customers/acme",
			"--payload", "id=acme,name=Acme,spec.tier=gold",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected apply output to be empty without --verbose, got %q", output)
		}
		if len(orchestrator.updateCalls) != 1 {
			t.Fatalf("expected explicit apply to perform update in test double, got %d update calls", len(orchestrator.updateCalls))
		}
		body, ok := orchestrator.updateCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected object payload, got %#v", orchestrator.updateCalls[0].value)
		}
		spec, ok := body["spec"].(map[string]any)
		if !ok || spec["tier"] != "gold" {
			t.Fatalf("expected dotted assignment payload to build nested object, got %#v", body)
		}
	})

	t.Run("create_rejects_identity_attribute_mismatch", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		_, err := executeForTest(
			deps,
			"",
			"resource", "create",
			"/customers/acme",
			"--payload", `{"id":"other","name":"Acme"}`,
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "does not match path segment") {
			t.Fatalf("expected explanatory mismatch error, got %v", err)
		}
		if len(orchestrator.createCalls) != 0 {
			t.Fatalf("expected create to be skipped on identity mismatch, got %#v", orchestrator.createCalls)
		}
	})

	t.Run("collection_path_explicit_payload_infers_resource_path_from_metadata_alias", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name    string
			command string
		}{
			{name: "create", command: "create"},
			{name: "apply", command: "apply"},
			{name: "update", command: "update"},
		}

		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				metadataService := newTestMetadata()
				metadataService.items["/admin/realms"] = metadatadomain.ResourceMetadata{
					IDFromAttribute:    "realm",
					AliasFromAttribute: "realm",
				}
				metadataService.wildcardChildren["/admin/realms"] = true

				orchestrator := &testOrchestrator{metadataService: metadataService}
				deps := testDepsWith(orchestrator, metadataService)

				output, err := executeForTest(
					deps,
					"",
					"resource", testCase.command,
					"/admin/realms",
					"--payload", "realm=test",
				)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if output != "" {
					t.Fatalf("expected %s output to be empty without --verbose, got %q", testCase.command, output)
				}

				const wantPath = "/admin/realms/test"
				switch testCase.command {
				case "create":
					if len(orchestrator.createCalls) != 1 || orchestrator.createCalls[0].logicalPath != wantPath {
						t.Fatalf("expected create target %q, got %#v", wantPath, orchestrator.createCalls)
					}
				case "apply":
					if len(orchestrator.getRemoteCalls) != 1 || orchestrator.getRemoteCalls[0] != wantPath {
						t.Fatalf("expected apply existence check on %q, got %#v", wantPath, orchestrator.getRemoteCalls)
					}
					if len(orchestrator.updateCalls) != 1 || orchestrator.updateCalls[0].logicalPath != wantPath {
						t.Fatalf("expected apply explicit payload update target %q, got %#v", wantPath, orchestrator.updateCalls)
					}
				case "update":
					if len(orchestrator.updateCalls) != 1 || orchestrator.updateCalls[0].logicalPath != wantPath {
						t.Fatalf("expected update target %q, got %#v", wantPath, orchestrator.updateCalls)
					}
				default:
					t.Fatalf("unexpected command %q", testCase.command)
				}
			})
		}
	})

	t.Run("create_explicit_resource_path_with_alias_metadata_still_works", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/test"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:    "realm",
			AliasFromAttribute: "realm",
		}
		metadataService.wildcardChildren["/admin/realms"] = true
		orchestrator := &testOrchestrator{metadataService: metadataService}
		deps := testDepsWith(orchestrator, metadataService)

		output, err := executeForTest(
			deps,
			"",
			"resource", "create",
			"/admin/realms/test",
			"--payload", "realm=test",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected create output to be empty without --verbose, got %q", output)
		}
		if len(orchestrator.createCalls) != 1 || orchestrator.createCalls[0].logicalPath != "/admin/realms/test" {
			t.Fatalf("expected create target to remain explicit resource path, got %#v", orchestrator.createCalls)
		}
	})

	t.Run("update_accepts_inline_json_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		output, err := executeForTest(
			deps,
			"",
			"resource", "update",
			"/customers/acme",
			"--payload", `{"id":"acme","name":"Updated"}`,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected update output to be empty without --verbose, got %q", output)
		}
		if len(orchestrator.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(orchestrator.updateCalls))
		}
		body, ok := orchestrator.updateCalls[0].value.(map[string]any)
		if !ok || body["name"] != "Updated" {
			t.Fatalf("expected inline update payload to be forwarded, got %#v", orchestrator.updateCalls[0].value)
		}
	})

	t.Run("save_accepts_dotted_assignment_payload", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)

		output, err := executeForTest(
			deps,
			"",
			"resource", "save",
			"/customers/acme",
			"--payload", "id=acme,name=Acme,spec.tier=gold",
			"--overwrite",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected save output to be empty without --verbose, got %q", output)
		}
		if len(orchestrator.saveCalls) != 1 {
			t.Fatalf("expected one save call, got %d", len(orchestrator.saveCalls))
		}
		body, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected object save payload, got %#v", orchestrator.saveCalls[0].value)
		}
		spec, ok := body["spec"].(map[string]any)
		if !ok || spec["tier"] != "gold" {
			t.Fatalf("expected dotted assignments to build nested object for save, got %#v", body)
		}
	})
}

func TestResourceSaveInputModes(t *testing.T) {
	t.Parallel()

	t.Run("override_alias_is_accepted", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/customers/acme",
			"--override",
		)
		if err != nil {
			t.Fatalf("unexpected error with --override alias: %v", err)
		}
	})

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

	t.Run("without_input_collection_marker_reads_remote_list_when_resource_get_is_not_found", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/user-registry/AD PRD/mappers"] = metadatadomain.ResourceMetadata{
			AliasFromAttribute: "name",
		}
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			getRemoteErr:    faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
			remoteList: []resource.Resource{
				{
					LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/alpha",
					Payload: map[string]any{
						"id":   "mapper-a",
						"name": "alpha",
					},
				},
				{
					LogicalPath: "/admin/realms/master/user-registry/AD PRD/mappers/beta",
					Payload: map[string]any{
						"id":   "mapper-b",
						"name": "beta",
					},
				},
			},
		}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			"",
			"resource",
			"save",
			"/admin/realms/master/user-registry/AD PRD/mappers/",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master/user-registry/AD PRD/mappers"}) {
			t.Fatalf("expected one remote list call for collection target, got %#v", orchestrator.listRemoteCalls)
		}
		if len(orchestrator.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote get call when collection list succeeds, got %#v", orchestrator.getRemoteCalls)
		}
		if len(orchestrator.saveCalls) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(orchestrator.saveCalls))
		}
		if orchestrator.saveCalls[0].logicalPath != "/admin/realms/master/user-registry/AD PRD/mappers/alpha" {
			t.Fatalf("expected first saved path /admin/realms/master/user-registry/AD PRD/mappers/alpha, got %q", orchestrator.saveCalls[0].logicalPath)
		}
		if orchestrator.saveCalls[1].logicalPath != "/admin/realms/master/user-registry/AD PRD/mappers/beta" {
			t.Fatalf("expected second saved path /admin/realms/master/user-registry/AD PRD/mappers/beta, got %q", orchestrator.saveCalls[1].logicalPath)
		}
	})

	t.Run("without_input_remote_list_falls_back_to_common_item_identity_attributes", func(t *testing.T) {
		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/master/clients"] = metadatadomain.ResourceMetadata{
			AliasFromAttribute: "clientId",
		}
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			remoteList: []resource.Resource{
				{
					LogicalPath: "/admin/realms/master/clients/app-a-id",
					Payload:     map[string]any{"id": "app-a-id", "enabled": true},
				},
				{
					LogicalPath: "/admin/realms/master/clients/app-b-id",
					Payload:     map[string]any{"id": "app-b-id", "enabled": false},
				},
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
		if len(orchestrator.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote get calls for explicit collection target, got %#v", orchestrator.getRemoteCalls)
		}
		if !reflect.DeepEqual(orchestrator.listRemoteCalls, []string{"/admin/realms/master/clients"}) {
			t.Fatalf("expected one remote list call, got %#v", orchestrator.listRemoteCalls)
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

	t.Run("stdin_saves_explicit_payload", func(t *testing.T) {
		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{metadataService: metadataService}

		deps := newResourceSaveDeps(orchestrator, metadataService)
		_, err := executeForTest(
			deps,
			`{"id":"acme","tier":"pro"}`,
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
		saved, ok := orchestrator.saveCalls[0].value.(map[string]any)
		if !ok {
			t.Fatalf("expected saved payload map, got %T", orchestrator.saveCalls[0].value)
		}
		if saved["id"] != "acme" || saved["tier"] != "pro" {
			t.Fatalf("expected payload from stdin to be saved, got %#v", saved)
		}
		if len(orchestrator.getRemoteCalls) != 0 {
			t.Fatalf("expected no remote reads when explicit input is provided, got %#v", orchestrator.getRemoteCalls)
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

func TestResourceSaveGitCommitMessages(t *testing.T) {
	t.Parallel()

	t.Run("git_context_commits_with_default_message", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "save",
			"/customers/acme",
			"--payload", `{"id":"acme","name":"Acme"}`,
			"--overwrite",
		)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "declarest: save resource /customers/acme" {
			t.Fatalf("unexpected commit message: %q", repoService.commitCalls[0])
		}
		if repoService.pushCalls != 0 {
			t.Fatalf("expected save auto-commit to avoid push, got %d push calls", repoService.pushCalls)
		}
	})

	t.Run("message_appends_to_default_commit_message", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "save",
			"/customers/acme",
			"--payload", `{"id":"acme"}`,
			"--overwrite",
			"--message", "ticket-123",
		)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "declarest: save resource /customers/acme - ticket-123" {
			t.Fatalf("unexpected appended commit message: %q", repoService.commitCalls[0])
		}
	})

	t.Run("message_override_replaces_default_commit_message", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "save",
			"/customers/acme",
			"--payload", `{"id":"acme"}`,
			"--overwrite",
			"--message-override", "custom commit",
		)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "custom commit" {
			t.Fatalf("unexpected override commit message: %q", repoService.commitCalls[0])
		}
	})

	t.Run("message_flags_are_mutually_exclusive", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "save",
			"/customers/acme",
			"--payload", `{"id":"acme"}`,
			"--overwrite",
			"--message", "suffix",
			"--message-override", "override",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--message") {
			t.Fatalf("expected message flag validation error, got %v", err)
		}
		if len(repoService.commitCalls) != 0 {
			t.Fatalf("expected no commit calls on validation failure, got %#v", repoService.commitCalls)
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

func TestResourceDeleteRequiresConfirmDelete(t *testing.T) {
	t.Parallel()

	_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme")
	assertTypedCategory(t, err, faults.ValidationError)
	if !strings.Contains(err.Error(), "flag --confirm-delete is required") {
		t.Fatalf("expected --confirm-delete validation message, got %v", err)
	}

	_, err = executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--confirm-delete")
	if err != nil {
		t.Fatalf("unexpected error with --confirm-delete: %v", err)
	}
}

func TestRepoPushHelpShowsForcePushFlag(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "repo", "push", "--help")
	if err != nil {
		t.Fatalf("expected repo push help output, got error: %v", err)
	}
	if !strings.Contains(output, "--force-push") {
		t.Fatalf("expected --force-push in repo push help output, got %q", output)
	}
	if strings.Contains(output, "--force ") {
		t.Fatalf("expected legacy --force alias to be hidden from repo push help output, got %q", output)
	}
}

func TestResourceDeleteSourceFlags(t *testing.T) {
	t.Parallel()

	t.Run("default_deletes_from_remote_server", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--confirm-delete")
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

	t.Run("source_repository_deletes_only_from_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--confirm-delete", "--source", "repository")
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

	t.Run("http_method_override_requires_remote_source", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--confirm-delete", "--source", "repository", "--http-method", "DELETE")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--http-method") {
			t.Fatalf("expected http-method/source validation error, got %v", err)
		}
	})

	t.Run("source_both_deletes_from_remote_and_repository", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		orchestrator := deps.Orchestrator.(*testOrchestrator)
		repositoryService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(deps, "", "resource", "delete", "/customers/acme", "--confirm-delete", "--source", "both")
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

	t.Run("legacy_source_flags_are_mutually_exclusive", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--confirm-delete", "--repository", "--both")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("source_and_legacy_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--confirm-delete", "--source", "repository", "--both")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("invalid_source_value_fails", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--confirm-delete", "--source", "invalid")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("legacy_force_alias_still_works", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--force")
		if err != nil {
			t.Fatalf("unexpected error with legacy --force alias: %v", err)
		}
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

	_, err := executeForTest(deps, "", "resource", "delete", "/customers", "--confirm-delete")
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
	_, err = executeForTest(deps, "", "resource", "delete", "/customers", "--confirm-delete", "--recursive")
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

	_, err := executeForTest(deps, "", "resource", "delete", "/orders", "--confirm-delete", "--recursive")
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

func TestResourceDeleteGitCommitMessages(t *testing.T) {
	t.Parallel()

	t.Run("legacy_repository_flag_commits_to_git_repo", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "delete",
			"/customers/acme",
			"--confirm-delete",
			"--repository",
		)
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(repoService.deleteCalls) != 1 {
			t.Fatalf("expected one repository delete call, got %d", len(repoService.deleteCalls))
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "declarest: delete resource /customers/acme" {
			t.Fatalf("unexpected commit message: %q", repoService.commitCalls[0])
		}
	})

	t.Run("message_appends_to_default_commit_message", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "delete",
			"/customers/acme",
			"--confirm-delete",
			"--source", "repository",
			"--message", "ticket-456",
		)
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "declarest: delete resource /customers/acme - ticket-456" {
			t.Fatalf("unexpected appended commit message: %q", repoService.commitCalls[0])
		}
	})

	t.Run("message_override_replaces_default_commit_message", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "delete",
			"/customers/acme",
			"--confirm-delete",
			"--source", "repository",
			"--message-override", "custom delete commit",
		)
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(repoService.commitCalls) != 1 {
			t.Fatalf("expected one commit call, got %d", len(repoService.commitCalls))
		}
		if repoService.commitCalls[0] != "custom delete commit" {
			t.Fatalf("unexpected override commit message: %q", repoService.commitCalls[0])
		}
	})

	t.Run("remote_only_delete_does_not_commit", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "delete",
			"/customers/acme",
			"--confirm-delete",
			"--source", "remote-server",
		)
		if err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if len(repoService.commitCalls) != 0 {
			t.Fatalf("expected no commit calls for remote-only delete, got %#v", repoService.commitCalls)
		}
	})

	t.Run("message_flags_are_mutually_exclusive", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		repoService := deps.ResourceStore.(*testRepository)

		_, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"resource", "delete",
			"/customers/acme",
			"--confirm-delete",
			"--source", "repository",
			"--message", "suffix",
			"--message-override", "override",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--message") {
			t.Fatalf("expected message flag validation error, got %v", err)
		}
		if len(repoService.commitCalls) != 0 {
			t.Fatalf("expected no commit calls on validation failure, got %#v", repoService.commitCalls)
		}
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

	t.Run("render_selector_defaults_operation_paths_when_metadata_path_is_missing", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			path         string
			operationArg string
			expectedPath string
		}{
			{
				name:         "list_defaults_to_dot",
				path:         "/admin/realms/_/clients/",
				operationArg: "",
				expectedPath: ".",
			},
			{
				name:         "get_defaults_to_dot_id_template",
				path:         "/admin/realms/_/clients/_",
				operationArg: "get",
				expectedPath: "./{{.id}}",
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				metadataService := newTestMetadata()
				metadataService.items[test.path] = metadatadomain.ResourceMetadata{}
				orchestrator := &testOrchestrator{metadataService: metadataService}

				args := []string{"metadata", "render", test.path}
				if strings.TrimSpace(test.operationArg) != "" {
					args = append(args, test.operationArg)
				}

				output, err := executeForTest(testDepsWith(orchestrator, metadataService), "", args...)
				if err != nil {
					t.Fatalf("unexpected selector render default-path error: %v", err)
				}
				if !strings.Contains(output, "\"path\": \""+test.expectedPath+"\"") {
					t.Fatalf("expected selector render default path %q, got %q", test.expectedPath, output)
				}
			})
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

	t.Run("get_selector_path_uses_direct_metadata_get", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.rejectSelectorPathInResolve = true
		metadataService.items["/admin/realms/_/user-registry"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:    "id",
			AliasFromAttribute: "name",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/_/user-registry",
		)
		if err != nil {
			t.Fatalf("unexpected selector metadata get error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"id\"") ||
			!strings.Contains(output, "\"aliasFromAttribute\": \"name\"") {
			t.Fatalf("expected selector metadata payload in output, got %q", output)
		}
	})

	t.Run("get_returns_full_metadata_with_defaults_by_default", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:    "realm",
			AliasFromAttribute: "realm",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"realm\"") {
			t.Fatalf("expected idFromAttribute override in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"getResource\"") ||
			!strings.Contains(output, "\"listCollection\"") {
			t.Fatalf("expected default operation entries in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"httpMethod\": \"GET\"") {
			t.Fatalf("expected canonical httpMethod field in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"httpHeaders\": [") {
			t.Fatalf("expected canonical httpHeaders field in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"Accept\"") ||
			!strings.Contains(output, "\"value\": \"application/json\"") {
			t.Fatalf("expected Accept header in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"Content-Type\"") {
			t.Fatalf("expected Content-Type header in metadata get output, got %q", output)
		}
		if strings.Contains(output, "\"accept\":") {
			t.Fatalf("expected metadata get output without accept field, got %q", output)
		}
		if strings.Contains(output, "\"contentType\":") {
			t.Fatalf("expected metadata get output without contentType field, got %q", output)
		}
		if !strings.Contains(output, "\"payload\": {") ||
			!strings.Contains(output, "\"filterAttributes\": null") ||
			!strings.Contains(output, "\"suppressAttributes\": []") ||
			!strings.Contains(output, "\"jqExpression\": \"\"") {
			t.Fatalf("expected canonical payload transform fields in metadata get output, got %q", output)
		}
		if strings.Contains(output, "\"ignoreAttributes\":") {
			t.Fatalf("expected metadata get output without compareResources ignoreAttributes alias, got %q", output)
		}
		if !strings.Contains(output, "\"path\": \"./{{.id}}\"") {
			t.Fatalf("expected default get resource path in metadata get output, got %q", output)
		}
	})

	t.Run("get_resolves_resource_format_templates_using_context_repository_format", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:    "realm",
			AliasFromAttribute: "realm",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"--context",
			"yaml",
			"-o",
			"json",
			"metadata",
			"get",
			"/admin/realms/",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get error: %v", err)
		}
		if !strings.Contains(output, "\"name\": \"Accept\"") ||
			!strings.Contains(output, "\"value\": \"application/yaml\"") {
			t.Fatalf("expected yaml Accept header in metadata get output, got %q", output)
		}
		if !strings.Contains(output, "\"name\": \"Content-Type\"") {
			t.Fatalf("expected yaml Content-Type header in metadata get output, got %q", output)
		}
		if strings.Contains(output, "\"accept\":") || strings.Contains(output, "\"contentType\":") {
			t.Fatalf("expected metadata get output without accept/contentType fields, got %q", output)
		}
	})

	t.Run("get_overrides_only_returns_compact_overrides", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		metadataService.items["/admin/realms/"] = metadatadomain.ResourceMetadata{
			IDFromAttribute:    "realm",
			AliasFromAttribute: "realm",
		}
		orchestrator := &testOrchestrator{metadataService: metadataService}

		output, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"get",
			"/admin/realms/",
			"--overrides-only",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get overrides-only error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"realm\"") {
			t.Fatalf("expected idFromAttribute override in metadata get overrides-only output, got %q", output)
		}
		if strings.Contains(output, "\"getResource\"") || strings.Contains(output, "\"listCollection\"") {
			t.Fatalf("expected overrides-only output without default operation entries, got %q", output)
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
		if !strings.Contains(output, "\"getResource\"") || !strings.Contains(output, "\"listCollection\"") {
			t.Fatalf("expected merged default operations in metadata get fallback output, got %q", output)
		}
		if !strings.Contains(output, "\"httpMethod\": \"GET\"") ||
			!strings.Contains(output, "\"payload\": {") {
			t.Fatalf("expected canonical metadata operation fields in fallback output, got %q", output)
		}
	})

	t.Run("get_overrides_only_not_found_falls_back_to_compact_infer", func(t *testing.T) {
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
			"--overrides-only",
		)
		if err != nil {
			t.Fatalf("unexpected metadata get overrides-only fallback error: %v", err)
		}
		if !strings.Contains(output, "\"idFromAttribute\": \"realm\"") {
			t.Fatalf("expected inferred idFromAttribute in metadata get overrides-only fallback output, got %q", output)
		}
		if strings.Contains(output, "\"getResource\"") || strings.Contains(output, "\"listCollection\"") {
			t.Fatalf("expected compact inferred metadata in overrides-only fallback output, got %q", output)
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

	t.Run("infer_recursive_is_not_implemented_and_does_not_persist", func(t *testing.T) {
		t.Parallel()

		metadataService := newTestMetadata()
		orchestrator := &testOrchestrator{
			metadataService: metadataService,
			openAPISpec: map[string]any{
				"paths": map[string]any{
					"/admin/realms": map[string]any{
						"get": map[string]any{},
					},
				},
			},
		}
		initialCount := len(metadataService.items)

		_, err := executeForTest(
			testDepsWith(orchestrator, metadataService),
			"",
			"metadata",
			"infer",
			"/admin/realms/",
			"--recursive",
			"--apply",
		)
		assertTypedCategory(t, err, faults.ValidationError)

		if len(metadataService.items) != initialCount {
			t.Fatalf("expected recursive infer to avoid persistence, metadata items=%#v", metadataService.items)
		}
		if _, found := metadataService.items["/admin/realms/"]; found {
			t.Fatalf("expected recursive infer to avoid setting metadata at /admin/realms/, got %#v", metadataService.items["/admin/realms/"])
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

	t.Run("get_rejects_structured_output_modes", func(t *testing.T) {
		t.Parallel()

		deps := testDeps()
		if _, err := executeForTest(deps, "", "secret", "store", "/customers/acme:apiToken", "token-123"); err != nil {
			t.Fatalf("store returned error: %v", err)
		}

		_, err := executeForTest(deps, "", "--output", "json", "secret", "get", "/customers/acme:apiToken")
		assertTypedCategory(t, err, faults.ValidationError)
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

	t.Run("git_context_verbose_outputs_worktree_details", func(t *testing.T) {
		t.Parallel()

		repoService := &testRepository{
			syncStatus: &repository.SyncReport{
				State:          repository.SyncStateNoRemote,
				Ahead:          0,
				Behind:         0,
				HasUncommitted: true,
			},
			worktreeStatus: []repository.WorktreeStatusEntry{
				{Path: "customers/acme.json", Staging: " ", Worktree: "M"},
				{Path: "customers/new.json", Staging: "?", Worktree: "?"},
			},
		}
		deps := testDeps()
		deps.ResourceStore = repoService
		deps.RepositorySync = repoService

		textOutput, err := executeForTest(deps, "", "--context", "git", "repo", "status", "--verbose")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(textOutput, "worktree:\n") {
			t.Fatalf("expected verbose worktree header, got %q", textOutput)
		}
		if !strings.Contains(textOutput, " M customers/acme.json") {
			t.Fatalf("expected modified file in verbose status output, got %q", textOutput)
		}
		if !strings.Contains(textOutput, "?? customers/new.json") {
			t.Fatalf("expected untracked file in verbose status output, got %q", textOutput)
		}
	})
}

func TestRepoCommitCommand(t *testing.T) {
	t.Parallel()

	t.Run("filesystem_context_fails_fast", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "repo", "commit", "--message", "manual changes")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "filesystem repositories") {
			t.Fatalf("expected filesystem-specific validation error, got %v", err)
		}
	})

	t.Run("git_context_commits_manual_changes", func(t *testing.T) {
		t.Parallel()

		repoService := &testRepository{}
		deps := testDeps()
		deps.ResourceStore = repoService
		deps.RepositorySync = repoService

		output, err := executeForTest(deps, "", "--context", "git", "repo", "commit", "--message", "manual changes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repoService.commitCalls) != 1 || repoService.commitCalls[0] != "manual changes" {
			t.Fatalf("expected one manual commit call, got %#v", repoService.commitCalls)
		}
		if output != "" {
			t.Fatalf("expected no text payload output for repo commit, got %q", output)
		}
	})

	t.Run("missing_message_fails_validation", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "--context", "git", "repo", "commit")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--message") {
			t.Fatalf("expected message validation error, got %v", err)
		}
	})

	t.Run("clean_worktree_returns_no_changes", func(t *testing.T) {
		t.Parallel()

		committed := false
		repoService := &testRepository{commitCommitted: &committed}
		deps := testDeps()
		deps.ResourceStore = repoService
		deps.RepositorySync = repoService

		output, err := executeForTest(deps, "", "--context", "git", "repo", "commit", "-m", "manual changes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output != "" {
			t.Fatalf("expected no text payload output for repo commit no-op, got %q", output)
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
		deps.RepositorySync = repoService

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

func TestRepoCleanCallsRepositorySync(t *testing.T) {
	t.Parallel()

	repoService := &testRepository{}
	deps := testDeps()
	deps.RepositorySync = repoService

	if _, err := executeForTest(deps, "", "repo", "clean"); err != nil {
		t.Fatalf("unexpected clean error: %v", err)
	}
	if repoService.cleanCalls != 1 {
		t.Fatalf("expected clean to be called once, got %d", repoService.cleanCalls)
	}
}

func TestRepoTreeCommand(t *testing.T) {
	t.Parallel()

	repoService := &testRepository{
		treeDirs: []string{
			"admin",
			"admin/realms",
			"admin/realms/acme",
			"admin/realms/acme/authentication",
			"admin/realms/acme/authentication/flows",
			"admin/realms/acme/authentication/flows/test",
			"admin/realms/acme/authentication/flows/test/executions",
			"admin/realms/acme/authentication/flows/test/executions/Cookie",
			"admin/realms/acme/clients",
			"admin/realms/acme/clients/test",
			"admin/realms/acme/organizations",
			"admin/realms/acme/organizations/alpha",
			"admin/realms/acme/user-registry",
			"admin/realms/acme/user-registry/AD PRD",
		},
	}
	deps := testDeps()
	deps.RepositorySync = repoService
	deps.ResourceStore = repoService

	output, err := executeForTest(deps, "", "repo", "tree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := strings.Join([]string{
		"admin",
		"└── realms",
		"    └── acme",
		"        ├── authentication",
		"        │   └── flows",
		"        │       └── test",
		"        │           └── executions",
		"        │               └── Cookie",
		"        ├── clients",
		"        │   └── test",
		"        ├── organizations",
		"        │   └── alpha",
		"        └── user-registry",
		"            └── AD PRD",
		"",
	}, "\n")
	if output != want {
		t.Fatalf("unexpected repo tree output:\n%s", output)
	}
	if repoService.treeCalls != 1 {
		t.Fatalf("expected tree to be called once, got %d", repoService.treeCalls)
	}
}

func TestRepoHistoryCommand(t *testing.T) {
	t.Parallel()

	t.Run("filesystem_context_reports_not_supported_message", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "repo", "history")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(strings.ToLower(output), "not supported") {
			t.Fatalf("expected not-supported message, got %q", output)
		}
	})

	t.Run("git_context_calls_history_and_applies_filters", func(t *testing.T) {
		t.Parallel()

		repoService := &testRepository{
			history: []repository.HistoryEntry{
				{
					Hash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Author:  "Alice",
					Email:   "alice@example.invalid",
					Date:    time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
					Subject: "fix customer sync",
				},
			},
		}
		deps := testDeps()
		deps.RepositorySync = repoService
		deps.ResourceStore = repoService

		output, err := executeForTest(
			deps,
			"",
			"--context", "git",
			"repo", "history",
			"--oneline",
			"--max-count", "5",
			"--author", "alice",
			"--grep", "fix",
			"--since", "2026-01-01",
			"--until", "2026-02-01",
			"--path", "customers",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "aaaaaaaaaaaa fix customer sync") {
			t.Fatalf("expected oneline history output, got %q", output)
		}

		if len(repoService.historyCalls) != 1 {
			t.Fatalf("expected one history call, got %d", len(repoService.historyCalls))
		}
		call := repoService.historyCalls[0]
		if call.MaxCount != 5 {
			t.Fatalf("expected max-count=5, got %d", call.MaxCount)
		}
		if call.Author != "alice" || call.Grep != "fix" {
			t.Fatalf("unexpected author/grep filters: %#v", call)
		}
		if len(call.Paths) != 1 || call.Paths[0] != "customers" {
			t.Fatalf("unexpected path filters: %#v", call.Paths)
		}
		if call.Since == nil || call.Until == nil {
			t.Fatalf("expected since/until to be parsed, got %#v", call)
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

func TestResourceListTextOutputUsesAliasAndID(t *testing.T) {
	t.Parallel()

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		remoteList: []resource.Resource{
			{
				LogicalPath: "/customers/acme",
				Metadata: metadatadomain.ResourceMetadata{
					AliasFromAttribute: "name",
					IDFromAttribute:    "id",
				},
				Payload: map[string]any{
					"id":   "42",
					"name": "acme",
				},
			},
			{
				LogicalPath: "/customers/beta",
				LocalAlias:  "beta",
				RemoteID:    "84",
				Payload:     map[string]any{"id": "84"},
			},
		},
	}

	output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "-o", "text", "resource", "list", "/customers")
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if strings.Contains(output, "/customers/acme") {
		t.Fatalf("expected alias/id list output instead of logical paths, got %q", output)
	}
	if !strings.Contains(output, "acme (42)") {
		t.Fatalf("expected metadata-derived alias/id output, got %q", output)
	}
	if !strings.Contains(output, "beta (84)") {
		t.Fatalf("expected resource identity output, got %q", output)
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

	t.Run("source_remote_server_lists_from_remote_server", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "resource", "list", "/", "--source", "remote-server")
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

	t.Run("source_repository_lists_from_repository", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList:       []resource.Resource{{LogicalPath: "/repo-only", Payload: map[string]any{"id": "repo-only"}}},
			remoteList:      []resource.Resource{{LogicalPath: "/remote-only", Payload: map[string]any{"id": "remote-only"}}},
		}
		output, err := executeForTest(testDepsWith(orchestrator, orchestrator.metadataService), "", "resource", "list", "/", "--source", "repository")
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

	t.Run("http_method_override_requires_remote_source", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "list", "/", "--source", "repository", "--http-method", "POST")
		assertTypedCategory(t, err, faults.ValidationError)
		if err == nil || !strings.Contains(err.Error(), "--http-method") {
			t.Fatalf("expected http-method/source validation error, got %v", err)
		}
	})

	t.Run("legacy_repository_and_remote_server_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "list", "/", "--repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("source_and_legacy_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "list", "/", "--source", "repository", "--remote-server")
		assertTypedCategory(t, err, faults.ValidationError)
	})

	t.Run("invalid_source_value_fails", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "list", "/", "--source", "both")
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

func TestResourceApplyUsesExplicitInputOverride(t *testing.T) {
	t.Parallel()

	t.Run("apply_with_payload_updates_existing_remote_resource", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "apply", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected apply error: %v", err)
		}
		if len(orchestrator.getRemoteCalls) != 1 || orchestrator.getRemoteCalls[0] != "/customers/acme" {
			t.Fatalf("expected apply explicit input to check remote existence, got %#v", orchestrator.getRemoteCalls)
		}
		if len(orchestrator.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(orchestrator.updateCalls))
		}
		if len(orchestrator.createCalls) != 0 {
			t.Fatalf("expected no create calls for existing resource, got %d", len(orchestrator.createCalls))
		}
		if len(orchestrator.applyCalls) != 0 {
			t.Fatalf("expected repository-driven apply path to be skipped, got %#v", orchestrator.applyCalls)
		}
		if output != "" {
			t.Fatalf("expected apply output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("apply_with_payload_creates_when_remote_resource_not_found", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			getRemoteValues: map[string]resource.Value{},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "apply", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected apply error: %v", err)
		}
		if len(orchestrator.createCalls) != 1 {
			t.Fatalf("expected one create call after remote not found, got %d", len(orchestrator.createCalls))
		}
		if len(orchestrator.updateCalls) != 0 {
			t.Fatalf("expected no update calls after remote not found, got %d", len(orchestrator.updateCalls))
		}
	})

	t.Run("apply_recursive_rejects_explicit_input", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, `{"id":"acme"}`, "resource", "apply", "/customers/acme", "--recursive")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceApplyRefreshesRepositoryExplicitInput(t *testing.T) {
	t.Parallel()

	orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	_, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "apply", "/customers/acme", "--refresh-repository")
	if err != nil {
		t.Fatalf("unexpected apply refresh error: %v", err)
	}
	if len(orchestrator.saveCalls) != 1 {
		t.Fatalf("expected one save call after refresh, got %d", len(orchestrator.saveCalls))
	}
	if orchestrator.saveCalls[0].logicalPath != "/customers/acme" {
		t.Fatalf("expected save target /customers/acme, got %q", orchestrator.saveCalls[0].logicalPath)
	}
}

func TestResourceApplyRefreshesRepositoryRecursive(t *testing.T) {
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

	_, err := executeForTest(deps, "", "resource", "apply", "/customers", "--recursive", "--refresh-repository")
	if err != nil {
		t.Fatalf("unexpected recursive apply refresh error: %v", err)
	}
	expectedPaths := []string{
		"/customers/acme",
		"/customers/beta",
		"/customers/nested/gamma",
	}
	if len(orchestrator.saveCalls) != len(expectedPaths) {
		t.Fatalf("expected %d save calls, got %d", len(expectedPaths), len(orchestrator.saveCalls))
	}
	for idx, expectedPath := range expectedPaths {
		if orchestrator.saveCalls[idx].logicalPath != expectedPath {
			t.Fatalf("expected save call %d to target %q, got %q", idx, expectedPath, orchestrator.saveCalls[idx].logicalPath)
		}
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

	t.Run("stdin_create_renders_no_output", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"startup"}`, "resource", "create", "/customers/acme")
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

	t.Run("stdin_create_verbose_renders_target", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"startup"}`, "resource", "create", "/customers/acme", "--verbose")
		if err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}
		if !strings.Contains(output, "/customers/acme") {
			t.Fatalf("expected create output to contain /customers/acme with --verbose, got %q", output)
		}
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

func TestResourceUpdateUsesExplicitInputOverride(t *testing.T) {
	t.Parallel()

	t.Run("update_with_payload", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, `{"id":"acme","tier":"pro"}`, "resource", "update", "/customers/acme")
		if err != nil {
			t.Fatalf("unexpected update error: %v", err)
		}
		if len(orchestrator.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(orchestrator.updateCalls))
		}
		if len(orchestrator.getLocalCalls) != 0 {
			t.Fatalf("expected repository payload lookups to be skipped, got %#v", orchestrator.getLocalCalls)
		}
		if output != "" {
			t.Fatalf("expected update output to be empty without --verbose, got %q", output)
		}
	})

	t.Run("update_recursive_rejects_explicit_input", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{metadataService: newTestMetadata()}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		_, err := executeForTest(deps, `{"id":"acme"}`, "resource", "update", "/customers/acme", "--recursive")
		assertTypedCategory(t, err, faults.ValidationError)
	})
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
					{ResourcePath: "/customers/acme", Path: "/name", Operation: "replace"},
				},
				"/customers/beta": {
					{ResourcePath: "/customers/beta", Path: "/enabled", Operation: "remove"},
				},
				"/customers/nested/gamma": {
					{ResourcePath: "/customers/nested/gamma", Path: "/name", Operation: "replace"},
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
		if !strings.Contains(output, "/customers/acme") || !strings.Contains(output, "/customers/beta") {
			t.Fatalf("expected diff output to include direct-child entries, got %q", output)
		}
		if strings.Contains(output, "/customers/nested/gamma") {
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
					{ResourcePath: idPath, Path: "/name", Operation: "replace"},
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
		var items []resource.DiffEntry
		if err := json.Unmarshal([]byte(output), &items); err != nil {
			t.Fatalf("failed to decode diff json output: %v", err)
		}
		if len(items) != 1 || items[0].ResourcePath != idPath || items[0].Path != "/name" {
			t.Fatalf("expected diff fallback output entry for %q with /name pointer, got %#v", idPath, items)
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
						ResourcePath: targetPath,
						Path:         "/attributes/clientOfflineSessionIdleTimeout",
						Operation:    "add",
						Local:        nil,
						Remote:       "0",
					},
					{
						ResourcePath: targetPath,
						Path:         "/displayName",
						Operation:    "replace",
						Local:        "Payments Realm",
						Remote:       "Payments Realm 2",
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

	t.Run("json_output_splits_resource_path_and_pointer_for_single_resource", func(t *testing.T) {
		t.Parallel()

		const targetPath = "/admin/realms/acme/authentication/flows/test/executions/Cookie"

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: targetPath},
			},
			diffValues: map[string][]resource.DiffEntry{
				targetPath: {
					{
						ResourcePath: targetPath,
						Path:         "/id",
						Operation:    "replace",
						Local:        "local-id",
						Remote:       "remote-id",
					},
					{
						ResourcePath: targetPath,
						Path:         "/requirement",
						Operation:    "replace",
						Local:        "ALTERNATIVE",
						Remote:       "DISABLED",
					},
				},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "diff", targetPath, "--output", "json")
		if err != nil {
			t.Fatalf("unexpected diff json output error: %v", err)
		}

		var items []resource.DiffEntry
		if err := json.Unmarshal([]byte(output), &items); err != nil {
			t.Fatalf("failed to decode diff json output: %v", err)
		}

		if len(items) != 2 {
			t.Fatalf("expected two diff entries, got %#v", items)
		}
		if items[0].ResourcePath != targetPath || items[0].Path != "/id" {
			t.Fatalf("expected first entry to split resource path and /id pointer, got %#v", items[0])
		}
		if items[1].ResourcePath != targetPath || items[1].Path != "/requirement" {
			t.Fatalf("expected second entry to split resource path and /requirement pointer, got %#v", items[1])
		}
	})

	t.Run("json_output_collection_is_sorted_by_resource_path_then_pointer", func(t *testing.T) {
		t.Parallel()

		orchestrator := &testOrchestrator{
			metadataService: newTestMetadata(),
			localList: []resource.Resource{
				{LogicalPath: "/customers/beta"},
				{LogicalPath: "/customers/acme"},
			},
			diffValues: map[string][]resource.DiffEntry{
				"/customers/beta": {
					{ResourcePath: "/customers/beta", Path: "/name", Operation: "replace"},
				},
				"/customers/acme": {
					{ResourcePath: "/customers/acme", Path: "/name", Operation: "replace"},
					{ResourcePath: "/customers/acme", Path: "/id", Operation: "replace"},
				},
			},
		}
		deps := testDepsWith(orchestrator, orchestrator.metadataService)

		output, err := executeForTest(deps, "", "resource", "diff", "/customers", "--output", "json")
		if err != nil {
			t.Fatalf("unexpected collection diff json error: %v", err)
		}

		var items []resource.DiffEntry
		if err := json.Unmarshal([]byte(output), &items); err != nil {
			t.Fatalf("failed to decode diff json output: %v", err)
		}

		if len(items) != 3 {
			t.Fatalf("expected three diff entries, got %#v", items)
		}
		expected := []struct {
			resourcePath string
			pointer      string
		}{
			{resourcePath: "/customers/acme", pointer: "/id"},
			{resourcePath: "/customers/acme", pointer: "/name"},
			{resourcePath: "/customers/beta", pointer: "/name"},
		}
		for idx, want := range expected {
			if items[idx].ResourcePath != want.resourcePath || items[idx].Path != want.pointer {
				t.Fatalf("expected item %d to be (%q,%q), got %#v", idx, want.resourcePath, want.pointer, items[idx])
			}
		}
	})
}

func TestResourceExplainTextOutputUsesJoinedResourceAndPointerPath(t *testing.T) {
	t.Parallel()

	const targetPath = "/admin/realms/acme/authentication/flows/test/executions/Cookie"

	orchestrator := &testOrchestrator{
		metadataService: newTestMetadata(),
		explainValues: map[string][]resource.DiffEntry{
			targetPath: {
				{ResourcePath: targetPath, Path: "", Operation: "replace"},
				{ResourcePath: targetPath, Path: "/requirement", Operation: "replace"},
			},
		},
	}
	deps := testDepsWith(orchestrator, orchestrator.metadataService)

	output, err := executeForTest(deps, "", "resource", "explain", targetPath, "--output", "text")
	if err != nil {
		t.Fatalf("unexpected explain text output error: %v", err)
	}

	expectedOutput := strings.Join([]string{
		"replace " + targetPath,
		"replace " + targetPath + "/requirement",
	}, "\n") + "\n"
	if output != expectedOutput {
		t.Fatalf("expected explain text output %q, got %q", expectedOutput, output)
	}
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

func TestCommandWithoutRequiredSubcommandShowsHelp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		args            []string
		expectedSnippet string
	}{
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
	if !strings.Contains(output, "Help about any command") {
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
	if !strings.Contains(output, "--payload") {
		t.Fatalf("expected --payload flag in resource save help output, got %q", output)
	}
	if !strings.Contains(output, "use '-' to read object from stdin") {
		t.Fatalf("expected --payload description to mention '-' stdin hint, got %q", output)
	}
	if !strings.Contains(output, "--overwrite") {
		t.Fatalf("expected --overwrite in resource save help output, got %q", output)
	}
	if strings.Contains(output, "--force ") {
		t.Fatalf("expected legacy --force alias to be hidden from resource save help output, got %q", output)
	}
}

func TestResourceMutationHelpShowsRepositoryFirstExamplesAndExplicitOverrideFlags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		args            []string
		repositoryFirst string
		explicitExample string
	}{
		{
			name:            "apply",
			args:            []string{"resource", "apply", "--help"},
			repositoryFirst: "declarest resource apply /customers/acme",
			explicitExample: "declarest resource apply /customers/acme --payload payload.json",
		},
		{
			name:            "create",
			args:            []string{"resource", "create", "--help"},
			repositoryFirst: "declarest resource create /customers/acme",
			explicitExample: "declarest resource create /customers/acme --payload payload.json",
		},
		{
			name:            "update",
			args:            []string{"resource", "update", "--help"},
			repositoryFirst: "declarest resource update /customers/acme",
			explicitExample: "declarest resource update /customers/acme --payload payload.json",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeForTest(testDeps(), "", testCase.args...)
			if err != nil {
				t.Fatalf("unexpected help error: %v", err)
			}
			if !strings.Contains(output, "--payload") {
				t.Fatalf("expected --payload in help output, got %q", output)
			}
			if !strings.Contains(output, testCase.repositoryFirst) {
				t.Fatalf("expected repository-first example %q, got %q", testCase.repositoryFirst, output)
			}
			if !strings.Contains(output, testCase.explicitExample) {
				t.Fatalf("expected explicit input example %q, got %q", testCase.explicitExample, output)
			}
			if strings.Index(output, testCase.repositoryFirst) > strings.Index(output, testCase.explicitExample) {
				t.Fatalf("expected repository example to appear before explicit input example in help output, got %q", output)
			}
			if !strings.Contains(strings.ToLower(output), "overrides repository input") {
				t.Fatalf("expected help text to mention explicit input override behavior, got %q", output)
			}
		})
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

func TestRootCompletionExcludesRemovedTopLevelRequestAlias(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "__complete", "")
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if strings.Contains(output, "\trequest\t") {
		t.Fatalf("expected request to remain nested under resource in root completion output, got %q", output)
	}
}

func TestCompletionBashScriptDoesNotContainRemovedAdHocCommand(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "completion", "bash")
	if err != nil {
		t.Fatalf("unexpected completion script error: %v", err)
	}
	if strings.Contains(output, "ad-hoc") {
		t.Fatalf("expected generated completion script to omit removed ad-hoc command, got %q", output)
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
