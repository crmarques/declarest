package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/reconciler"
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

func executeForTest(deps common.CommandWiring, stdin string, args ...string) (string, error) {
	command := NewRootCommand(deps)
	out := &bytes.Buffer{}
	command.SetOut(out)
	command.SetErr(io.Discard)
	command.SetIn(strings.NewReader(stdin))
	command.SetArgs(args)

	err := command.Execute()
	return out.String(), err
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

func testDeps() common.CommandWiring {
	metadataService := newTestMetadataService()
	secretProvider := newTestSecretProvider()

	return common.CommandWiring{
		Reconciler: &testReconciler{
			metadataService: metadataService,
			secretProvider:  secretProvider,
		},
		Contexts: &testContextService{},
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
	metadataService *testMetadataService
	secretProvider  *testSecretProvider
}

func (r *testReconciler) Get(_ context.Context, logicalPath string) (resource.Value, error) {
	return map[string]any{"path": logicalPath}, nil
}
func (r *testReconciler) Save(context.Context, string, resource.Value) error { return nil }
func (r *testReconciler) Apply(_ context.Context, logicalPath string) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Create(_ context.Context, logicalPath string, _ resource.Value) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Update(_ context.Context, logicalPath string, _ resource.Value) (resource.Resource, error) {
	return resource.Resource{LogicalPath: logicalPath}, nil
}
func (r *testReconciler) Delete(context.Context, string, reconciler.DeletePolicy) error { return nil }
func (r *testReconciler) ListLocal(_ context.Context, logicalPath string, policy reconciler.ListPolicy) ([]resource.Resource, error) {
	if policy.Recursive {
		return []resource.Resource{{LogicalPath: logicalPath + "/nested"}}, nil
	}
	return []resource.Resource{{LogicalPath: logicalPath}}, nil
}
func (r *testReconciler) ListRemote(_ context.Context, logicalPath string, policy reconciler.ListPolicy) ([]resource.Resource, error) {
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
func (r *testReconciler) RepoInit(context.Context) error                          { return nil }
func (r *testReconciler) RepoRefresh(context.Context) error                       { return nil }
func (r *testReconciler) RepoPush(context.Context, repository.PushPolicy) error   { return nil }
func (r *testReconciler) RepoReset(context.Context, repository.ResetPolicy) error { return nil }
func (r *testReconciler) RepoCheck(context.Context) error                         { return nil }
func (r *testReconciler) RepoStatus(context.Context) (repository.SyncReport, error) {
	return repository.SyncReport{
		State:          repository.SyncStateNoRemote,
		Ahead:          0,
		Behind:         0,
		HasUncommitted: false,
	}, nil
}

func (r *testReconciler) MetadataManager() metadatadomain.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadataService
}

func (r *testReconciler) SecretManager() secretdomain.SecretProvider {
	if r == nil {
		return nil
	}
	return r.secretProvider
}

type testMetadataService struct {
	items map[string]metadatadomain.ResourceMetadata
}

func newTestMetadataService() *testMetadataService {
	return &testMetadataService{
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

func (s *testMetadataService) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	metadata, found := s.items[logicalPath]
	if !found {
		return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
	}
	return metadata, nil
}

func (s *testMetadataService) Set(_ context.Context, logicalPath string, metadata metadatadomain.ResourceMetadata) error {
	s.items[logicalPath] = metadata
	return nil
}

func (s *testMetadataService) Unset(_ context.Context, logicalPath string) error {
	delete(s.items, logicalPath)
	return nil
}

func (s *testMetadataService) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if metadata, found := s.items[logicalPath]; found {
		return metadata, nil
	}
	return metadatadomain.ResourceMetadata{}, nil
}

func (s *testMetadataService) RenderOperationSpec(
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

func (s *testMetadataService) Infer(
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
