package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
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
		"config",
		"config create",
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

	t.Run("local_flag_uses_local", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--local")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"local\"") {
			t.Fatalf("expected local source output, got %q", output)
		}
	})

	t.Run("remote_flag_uses_remote", func(t *testing.T) {
		t.Parallel()

		output, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--remote")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "\"source\": \"remote\"") {
			t.Fatalf("expected remote source output, got %q", output)
		}
	})

	t.Run("local_and_remote_flags_conflict", func(t *testing.T) {
		t.Parallel()

		_, err := executeForTest(testDeps(), "", "resource", "get", "/customers/acme", "--local", "--remote")
		assertTypedCategory(t, err, faults.ValidationError)
	})
}

func TestResourceSaveInputModes(t *testing.T) {
	t.Parallel()

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

	output, err := executeForTest(testDeps(), "", "resource", "delete", "/customers/acme")
	if err != nil {
		t.Fatalf("expected help output when --force is missing, got error: %v", err)
	}
	if !strings.Contains(output, "confirm deletion") {
		t.Fatalf("expected delete help output, got %q", output)
	}

	_, err = executeForTest(testDeps(), "", "resource", "delete", "/customers/acme", "--force")
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
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
	if !strings.Contains(directOutput, "\"LogicalPath\": \"/customers\"") {
		t.Fatalf("expected direct list payload, got %q", directOutput)
	}

	recursiveOutput, err := executeForTest(testDeps(), "", "resource", "list", "/customers", "--recursive")
	if err != nil {
		t.Fatalf("unexpected recursive list error: %v", err)
	}
	if !strings.Contains(recursiveOutput, "\"LogicalPath\": \"/customers/nested\"") {
		t.Fatalf("expected recursive list payload, got %q", recursiveOutput)
	}
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

func TestCommandWithoutRequiredSubcommandShowsHelp(t *testing.T) {
	t.Parallel()

	output, err := executeForTest(testDeps(), "", "resource")
	if err != nil {
		t.Fatalf("expected help output for missing subcommand, got error: %v", err)
	}
	if !strings.Contains(output, "Manage resources") {
		t.Fatalf("expected resource help output, got %q", output)
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	t.Parallel()

	_, err := executeForTest(testDeps(), "", "unknown-command")
	if err == nil {
		t.Fatal("expected unknown command to return error")
	}
}

func TestHelpSubcommandDisabled(t *testing.T) {
	t.Parallel()

	_, err := executeForTest(testDeps(), "", "help")
	if err == nil {
		t.Fatal("expected help subcommand to be disabled")
	}

	output, err := executeForTest(testDeps(), "", "resource", "--help")
	if err != nil {
		t.Fatalf("expected --help flag to work: %v", err)
	}
	if !strings.Contains(output, "Read a resource") {
		t.Fatalf("expected command help output, got %q", output)
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
	saveErr         error
}

type savedResource struct {
	logicalPath string
	value       resource.Value
}

func (r *testReconciler) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath, "source": "get"}, nil
}
func (r *testReconciler) GetLocal(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath, "source": "local"}, nil
}
func (r *testReconciler) GetRemote(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath, "source": "remote"}, nil
}
func (r *testReconciler) Save(_ context.Context, logicalPath string, value resource.Value) error {
	r.saveCalls = append(r.saveCalls, savedResource{
		logicalPath: logicalPath,
		value:       value,
	})
	return r.saveErr
}
func (r *testReconciler) Apply(_ context.Context, logicalPath string) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Create(_ context.Context, logicalPath string, _ resource.Value) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Update(_ context.Context, logicalPath string, _ resource.Value) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Delete(context.Context, string, orchestrator.DeletePolicy) error { return nil }
func (r *testReconciler) ListLocal(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	if policy.Recursive {
		return []resource.Resource{{LogicalPath: logicalPath + "/nested"}}, nil
	}
	return []resource.Resource{{LogicalPath: logicalPath}}, nil
}
func (r *testReconciler) ListRemote(_ context.Context, logicalPath string, policy orchestrator.ListPolicy) ([]resource.Resource, error) {
	if policy.Recursive {
		return []resource.Resource{{LogicalPath: logicalPath + "/nested"}}, nil
	}
	return []resource.Resource{{LogicalPath: logicalPath}}, nil
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

type testRepository struct{}

func (r *testRepository) Save(context.Context, string, resource.Value) error { return nil }
func (r *testRepository) Get(context.Context, string) (resource.Value, error) {
	return map[string]any{"id": "acme"}, nil
}
func (r *testRepository) Delete(context.Context, string, repository.DeletePolicy) error { return nil }
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
