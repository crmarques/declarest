package cli

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	clitestkit "github.com/crmarques/declarest/internal/cli/testkit"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func executeForTest(deps Dependencies, stdin string, args ...string) (string, error) {
	return clitestkit.ExecuteCommandForTest(NewRootCommand(deps), stdin, args...)
}

func executeForTestWithStreams(deps Dependencies, stdin string, args ...string) (string, string, error) {
	return clitestkit.ExecuteCommandForTestWithStreams(NewRootCommand(deps), stdin, args...)
}

func registeredPaths(command *cobra.Command, prefix []string) [][]string {
	return clitestkit.RegisteredPaths(command, prefix)
}

func joinPath(path []string) string {
	return clitestkit.JoinPath(path)
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
	resourceServer := &testResourceServer{accessToken: "test-access-token"}

	return Dependencies{
		Orchestrator:   orchestrator,
		Contexts:       &testContextService{},
		ResourceStore:  repositoryService,
		RepositorySync: repositoryService,
		Metadata:       metadataService,
		Secrets:        secretProvider,
		ResourceServer: resourceServer,
	}
}

func newResourceSaveDeps(orchestrator *testOrchestrator, metadataService *testMetadata) Dependencies {
	deps := testDepsWith(orchestrator, metadataService)
	repositoryService := &resourceSaveTestRepository{}
	deps.ResourceStore = repositoryService
	deps.RepositorySync = repositoryService
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

	resourceServerAuth := &config.HTTPAuth{
		OAuth2: &config.OAuth2{
			TokenURL:     "https://auth.example.invalid/oauth/token",
			GrantType:    config.OAuthClientCreds,
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
	}
	if name == "bearer" {
		resourceServerAuth = &config.HTTPAuth{
			BearerToken: &config.BearerTokenAuth{Token: "bearer-token"},
		}
	}

	return config.Context{
		Name:       name,
		Repository: repositoryConfig,
		ResourceServer: &config.ResourceServer{
			HTTP: &config.HTTPServer{
				BaseURL: "https://api.example.invalid",
				Auth:    resourceServerAuth,
			},
		},
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
	requestCalls     []requestCall
	requestErr       error
	getLocalCalls    []string
	listLocalCalls   []string
	listLocalDetail  []listCall
	listRemoteCalls  []string
	listRemoteDetail []listCall
	listRemoteErr    error
	applyCalls       []string
	createCalls      []savedResource
	updateCalls      []savedResource
	explainCalls     []string
	diffCalls        []string
	explainValues    map[string][]resource.DiffEntry
	diffValues       map[string][]resource.DiffEntry
	explainErr       error
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

type requestCall struct {
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
func (r *testOrchestrator) Request(_ context.Context, method string, endpointPath string, body resource.Value) (resource.Value, error) {
	r.requestCalls = append(r.requestCalls, requestCall{
		method: method,
		path:   endpointPath,
		body:   body,
	})
	if r.requestErr != nil {
		return nil, r.requestErr
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
	r.explainCalls = append(r.explainCalls, logicalPath)
	if r.explainErr != nil {
		return nil, r.explainErr
	}
	if r.explainValues != nil {
		if value, ok := r.explainValues[logicalPath]; ok {
			items := make([]resource.DiffEntry, len(value))
			copy(items, value)
			return items, nil
		}
	}
	return []resource.DiffEntry{{ResourcePath: logicalPath, Path: "", Operation: "noop"}}, nil
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
	return []resource.DiffEntry{{ResourcePath: logicalPath, Path: "", Operation: "noop"}}, nil
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
	items                       map[string]metadatadomain.ResourceMetadata
	collectionChildren          map[string][]string
	wildcardChildren            map[string]bool
	rejectSelectorPathInResolve bool
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
		collectionChildren: map[string][]string{},
		wildcardChildren:   map[string]bool{},
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
	if s.rejectSelectorPathInResolve && strings.Contains(logicalPath, "/_/") {
		return metadatadomain.ResourceMetadata{}, faults.NewTypedError(
			faults.ValidationError,
			"logical path must not contain reserved metadata segment \"_\"",
			nil,
		)
	}
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

func (s *testMetadata) ResolveCollectionChildren(_ context.Context, logicalPath string) ([]string, error) {
	children, found := s.collectionChildren[logicalPath]
	if !found {
		return nil, nil
	}
	items := make([]string, len(children))
	copy(items, children)
	return items, nil
}

func (s *testMetadata) HasCollectionWildcardChild(_ context.Context, logicalPath string) (bool, error) {
	if s.wildcardChildren == nil {
		return false, nil
	}
	return s.wildcardChildren[logicalPath], nil
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

type testResourceServer struct {
	accessToken string
	tokenErr    error
}

func (s *testResourceServer) Get(context.Context, resource.Resource) (resource.Value, error) {
	return map[string]any{"ok": true}, nil
}

func (s *testResourceServer) Create(context.Context, resource.Resource) (resource.Value, error) {
	return map[string]any{"ok": true}, nil
}

func (s *testResourceServer) Update(context.Context, resource.Resource) (resource.Value, error) {
	return map[string]any{"ok": true}, nil
}

func (s *testResourceServer) Delete(context.Context, resource.Resource) error { return nil }

func (s *testResourceServer) List(context.Context, string, metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
	return nil, nil
}

func (s *testResourceServer) Exists(context.Context, resource.Resource) (bool, error) {
	return true, nil
}

func (s *testResourceServer) Request(context.Context, string, string, resource.Value) (resource.Value, error) {
	return map[string]any{"ok": true}, nil
}

func (s *testResourceServer) GetOpenAPISpec(context.Context) (resource.Value, error) { return nil, nil }

func (s *testResourceServer) GetAccessToken(context.Context) (string, error) {
	if s.tokenErr != nil {
		return "", s.tokenErr
	}
	if s.accessToken == "" {
		return "", faults.NewTypedError(faults.ValidationError, "resource-server.http.auth.oauth2 is not configured", nil)
	}
	return s.accessToken, nil
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

func assertOperationHTTPHeaderValue(t *testing.T, operation map[string]any, headerName string, expectedValue string) {
	t.Helper()

	rawHeaders, found := operation["httpHeaders"]
	if !found {
		t.Fatalf("expected httpHeaders list, got %#v", operation)
	}
	headers, ok := rawHeaders.([]any)
	if !ok {
		t.Fatalf("expected httpHeaders array, got %#v", rawHeaders)
	}

	for _, item := range headers {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["name"] == headerName && entry["value"] == expectedValue {
			return
		}
	}

	t.Fatalf("expected httpHeaders to contain %q=%q, got %#v", headerName, expectedValue, rawHeaders)
}
