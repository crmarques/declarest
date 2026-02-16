package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestDefaultOrchestratorDelegatesRepositoryMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepository{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
	}

	reconciler := &DefaultOrchestrator{
		Repository: fakeRepo,
	}

	value, err := reconciler.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value == nil {
		t.Fatal("expected non-nil value")
	}

	if err := reconciler.Save(context.Background(), "/customers/acme", map[string]any{"x": 1}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if err := reconciler.Delete(context.Background(), "/customers", DeletePolicy{Recursive: true}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	items, err := reconciler.ListLocal(context.Background(), "/customers", ListPolicy{Recursive: true})
	if err != nil {
		t.Fatalf("ListLocal returned error: %v", err)
	}
	if len(items) != 1 || items[0].LogicalPath != "/customers/acme" {
		t.Fatalf("unexpected list output: %#v", items)
	}

	if !fakeRepo.deletePolicy.Recursive {
		t.Fatal("expected delete policy recursion to be mapped")
	}
	if !fakeRepo.listPolicy.Recursive {
		t.Fatal("expected list policy recursion to be mapped")
	}
}

func TestDefaultOrchestratorRequiresRepository(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{}

	_, err := reconciler.Get(context.Background(), "/customers/acme")
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.ValidationError)
}

func TestDefaultOrchestratorGetFallsBackToRemoteWhenLocalMissing(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{
		Repository: &fakeRepository{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		},
		Metadata: &fakeMetadata{
			resolveErr: faults.NewTypedError(faults.NotFoundError, "metadata not found", nil),
		},
		Server: &fakeServer{
			getValue: map[string]any{"realm": "master"},
		},
	}

	value, err := reconciler.Get(context.Background(), "/admin/realms/")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	serverManager := reconciler.Server.(*fakeServer)
	if !serverManager.getCalled {
		t.Fatal("expected remote fallback get call")
	}
	if got := serverManager.lastResource.LogicalPath; got != "/admin/realms" {
		t.Fatalf("expected normalized remote logical path /admin/realms, got %q", got)
	}
	if got := serverManager.lastResource.CollectionPath; got != "/admin" {
		t.Fatalf("expected remote collection path /admin, got %q", got)
	}
	if !reflect.DeepEqual(value, map[string]any{"realm": "master"}) {
		t.Fatalf("unexpected remote payload: %#v", value)
	}
}

func TestDefaultOrchestratorGetRemoteFallbackSeedsIdentityFromMetadata(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{
		Repository: &fakeRepository{
			getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		},
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				IDFromAttribute:    "realm",
				AliasFromAttribute: "realm",
			},
		},
		Server: &fakeServer{
			getValue: map[string]any{"realm": "platform"},
		},
	}

	_, err := reconciler.Get(context.Background(), "/admin/realms/platform")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	serverManager := reconciler.Server.(*fakeServer)
	payload, ok := serverManager.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", serverManager.lastResource.Payload)
	}
	if got := payload["realm"]; got != "platform" {
		t.Fatalf("expected metadata-seeded realm identity, got %#v", got)
	}
	if got := serverManager.lastResource.RemoteID; got != "platform" {
		t.Fatalf("expected remote id platform, got %q", got)
	}
}

type fakeRepository struct {
	getValue    resource.Value
	getErr      error
	listValue   []resource.Resource
	statusValue repository.SyncReport

	savedPath  string
	savedValue resource.Value

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func (f *fakeRepository) Save(_ context.Context, logicalPath string, value resource.Value) error {
	f.savedPath = logicalPath
	f.savedValue = value
	return nil
}

func (f *fakeRepository) Get(context.Context, string) (resource.Value, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeRepository) Delete(_ context.Context, _ string, policy repository.DeletePolicy) error {
	f.deletePolicy = policy
	return nil
}

func (f *fakeRepository) List(_ context.Context, _ string, policy repository.ListPolicy) ([]resource.Resource, error) {
	f.listPolicy = policy
	return f.listValue, nil
}

func (f *fakeRepository) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeRepository) Move(context.Context, string, string) error   { return nil }
func (f *fakeRepository) Init(context.Context) error                   { return nil }
func (f *fakeRepository) Refresh(context.Context) error                { return nil }
func (f *fakeRepository) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (f *fakeRepository) Check(context.Context) error { return nil }
func (f *fakeRepository) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (f *fakeRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return f.statusValue, nil
}

type fakeMetadata struct {
	resolveValue metadatadomain.ResourceMetadata
	resolveErr   error
}

func (f *fakeMetadata) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

func (f *fakeMetadata) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f *fakeMetadata) Unset(context.Context, string) error {
	return nil
}

func (f *fakeMetadata) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	if f.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, f.resolveErr
	}
	return f.resolveValue, nil
}

func (f *fakeMetadata) RenderOperationSpec(
	ctx context.Context,
	_ string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.ResolveOperationSpec(ctx, f.resolveValue, operation, value)
}

func (f *fakeMetadata) Infer(context.Context, string, metadatadomain.InferenceRequest) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

type fakeServer struct {
	getValue    resource.Value
	getErr      error
	createValue resource.Value
	updateValue resource.Value
	listValue   []resource.Resource
	listErr     error
	existsValue bool
	existsErr   error

	createCalled bool
	updateCalled bool
	getCalled    bool
	listCalled   bool
	lastResource resource.Resource
}

func (f *fakeServer) Get(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.getCalled = true
	f.lastResource = resourceInfo
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeServer) Create(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.createCalled = true
	f.lastResource = resourceInfo
	return f.createValue, nil
}

func (f *fakeServer) Update(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.updateCalled = true
	f.lastResource = resourceInfo
	return f.updateValue, nil
}

func (f *fakeServer) Delete(context.Context, resource.Resource) error {
	return nil
}

func (f *fakeServer) List(context.Context, string, metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
	f.listCalled = true
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listValue, nil
}

func (f *fakeServer) Exists(context.Context, resource.Resource) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.existsValue, nil
}

func (f *fakeServer) GetOpenAPISpec(context.Context) (resource.Value, error) {
	return nil, nil
}

func (f *fakeServer) BuildRequestFromMetadata(context.Context, resource.Resource, metadatadomain.Operation) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

type fakeSecretProvider struct {
	values map[string]string
}

func (f *fakeSecretProvider) Init(context.Context) error { return nil }

func (f *fakeSecretProvider) Store(_ context.Context, key string, value string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value
	return nil
}

func (f *fakeSecretProvider) Get(_ context.Context, key string) (string, error) {
	value, found := f.values[key]
	if !found {
		return "", faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("secret %q not found", key), nil)
	}
	return value, nil
}

func (f *fakeSecretProvider) Delete(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func (f *fakeSecretProvider) List(context.Context) ([]string, error) { return nil, nil }

func (f *fakeSecretProvider) MaskPayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.MaskPayload(value, func(key string, secretValue string) error {
		return f.Store(ctx, key, secretValue)
	})
}

func (f *fakeSecretProvider) ResolvePayload(ctx context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.ResolvePayload(value, func(key string) (string, error) {
		return f.Get(ctx, key)
	})
}

func (f *fakeSecretProvider) NormalizeSecretPlaceholders(_ context.Context, value resource.Value) (resource.Value, error) {
	return secretdomain.NormalizePlaceholders(value)
}

func (f *fakeSecretProvider) DetectSecretCandidates(_ context.Context, value resource.Value) ([]string, error) {
	return secretdomain.DetectSecretCandidates(value)
}

func TestDefaultOrchestratorApplyUsesSecretsAndPersistsMaskedPayload(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":       "42",
			"alias":    "acme",
			"apiToken": "{{ secret \"apiToken\" }}",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationCreate): {Path: "/api/customers"},
		},
	}
	metadataService := &fakeMetadata{resolveValue: md}

	serverManager := &fakeServer{
		existsValue: true,
		updateValue: map[string]any{
			"id":       "42",
			"alias":    "acme",
			"apiToken": "super-secret",
		},
	}

	secretProvider := &fakeSecretProvider{
		values: map[string]string{
			"apiToken": "super-secret",
		},
	}

	reconciler := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
		Secrets:    secretProvider,
	}

	item, err := reconciler.Apply(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	if !serverManager.updateCalled || serverManager.createCalled {
		t.Fatalf("expected update mutation, got create=%t update=%t", serverManager.createCalled, serverManager.updateCalled)
	}

	updatePayload, ok := serverManager.lastResource.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected update payload map, got %T", serverManager.lastResource.Payload)
	}
	if got := updatePayload["apiToken"]; got != "super-secret" {
		t.Fatalf("expected resolved secret for remote payload, got %#v", got)
	}

	savedPayload, ok := repo.savedValue.(map[string]any)
	if !ok {
		t.Fatalf("expected saved payload map, got %T", repo.savedValue)
	}
	if got := savedPayload["apiToken"]; got != "{{secret \"apiToken\"}}" {
		t.Fatalf("expected masked local secret placeholder, got %#v", got)
	}
	if repo.savedPath != "/customers/acme" {
		t.Fatalf("expected save path /customers/acme, got %q", repo.savedPath)
	}

	if !reflect.DeepEqual(item.Payload, repo.savedValue) {
		t.Fatalf("expected returned payload to match persisted payload, got %#v", item.Payload)
	}
}

func TestDefaultOrchestratorDiffUsesFallbackAndCompareSuppressRules(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":        "42",
			"alias":     "acme",
			"name":      "ACME",
			"apiToken":  "{{ secret \"apiToken\" }}",
			"updatedAt": "2026-02-10T10:00:00Z",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Suppress: []string{"/updatedAt"}},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{
				LogicalPath: "/customers/acme",
				LocalAlias:  "acme",
				RemoteID:    "42",
				Payload: map[string]any{
					"id":        "42",
					"alias":     "acme",
					"name":      "ACME",
					"apiToken":  "super-secret",
					"updatedAt": "2026-02-11T10:00:00Z",
				},
			},
		},
	}

	secretProvider := &fakeSecretProvider{
		values: map[string]string{
			"apiToken": "super-secret",
		},
	}

	reconciler := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
		Secrets:    secretProvider,
	}

	items, err := reconciler.Diff(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}

	if !serverManager.getCalled || !serverManager.listCalled {
		t.Fatalf("expected bounded fallback flow, get=%t list=%t", serverManager.getCalled, serverManager.listCalled)
	}
	if len(items) != 0 {
		t.Fatalf("expected no drift after compare transforms, got %#v", items)
	}
}

func TestDefaultOrchestratorDiffReturnsConflictOnAmbiguousFallback(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		getValue: map[string]any{
			"id":    "42",
			"alias": "acme",
		},
	}

	md := metadatadomain.ResourceMetadata{
		IDFromAttribute:    "id",
		AliasFromAttribute: "alias",
		Operations: map[string]metadatadomain.OperationSpec{
			string(metadatadomain.OperationGet):     {Path: "/api/customers/{{.id}}"},
			string(metadatadomain.OperationList):    {Path: "/api/customers"},
			string(metadatadomain.OperationCompare): {Path: "/api/customers/{{.id}}"},
		},
	}

	metadataService := &fakeMetadata{resolveValue: md}
	serverManager := &fakeServer{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme-1", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
			{LogicalPath: "/customers/acme-2", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
		},
	}

	reconciler := &DefaultOrchestrator{
		Repository: repo,
		Metadata:   metadataService,
		Server:     serverManager,
	}

	_, err := reconciler.Diff(context.Background(), "/customers/acme")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestDefaultOrchestratorListRemoteSortsDeterministically(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{
		Repository: &fakeRepository{},
		Metadata:   &fakeMetadata{resolveValue: metadatadomain.ResourceMetadata{}},
		Server: &fakeServer{
			listValue: []resource.Resource{
				{LogicalPath: "/customers/zeta"},
				{LogicalPath: "/customers/acme"},
			},
		},
	}

	items, err := reconciler.ListRemote(context.Background(), "/customers", ListPolicy{})
	if err != nil {
		t.Fatalf("ListRemote returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].LogicalPath != "/customers/acme" || items[1].LogicalPath != "/customers/zeta" {
		t.Fatalf("expected deterministic order, got %#v", items)
	}
}

func TestDefaultOrchestratorTemplateReturnsNormalizedPayload(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{
		Repository: &fakeRepository{},
		Metadata: &fakeMetadata{
			resolveValue: metadatadomain.ResourceMetadata{
				Operations: map[string]metadatadomain.OperationSpec{
					string(metadatadomain.OperationUpdate): {Path: "/api/customers/{{.id}}"},
				},
			},
		},
	}

	templated, err := reconciler.Template(context.Background(), "/customers/acme", map[string]any{
		"id":    "42",
		"name":  "ACME",
		"count": float64(2),
	})
	if err != nil {
		t.Fatalf("Template returned error: %v", err)
	}

	output, ok := templated.(map[string]any)
	if !ok {
		t.Fatalf("expected templated map output, got %T", templated)
	}
	if got := output["name"]; got != "ACME" {
		t.Fatalf("expected templated payload to preserve values, got %#v", got)
	}
}

func TestDefaultOrchestratorRenderOperationSpecListUsesCollectionPathFallback(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{}
	resourceInfo := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
		Metadata: metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{},
		},
	}

	spec, err := reconciler.renderOperationSpec(
		context.Background(),
		resourceInfo,
		metadatadomain.OperationList,
		map[string]any{"realm": "platform", "clientId": "declarest-cli"},
	)
	if err != nil {
		t.Fatalf("renderOperationSpec returned error: %v", err)
	}

	if spec.Path != "/admin/realms/platform/clients" {
		t.Fatalf("expected list fallback path /admin/realms/platform/clients, got %q", spec.Path)
	}
}

func TestDefaultOrchestratorRenderOperationSpecCreateUsesLogicalPathFallback(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultOrchestrator{}
	resourceInfo := resource.Resource{
		LogicalPath:    "/admin/realms/platform/clients/declarest-cli/resource",
		CollectionPath: "/admin/realms/platform/clients",
		Metadata: metadatadomain.ResourceMetadata{
			Operations: map[string]metadatadomain.OperationSpec{},
		},
	}

	spec, err := reconciler.renderOperationSpec(
		context.Background(),
		resourceInfo,
		metadatadomain.OperationCreate,
		map[string]any{"realm": "platform", "clientId": "declarest-cli"},
	)
	if err != nil {
		t.Fatalf("renderOperationSpec returned error: %v", err)
	}

	if spec.Path != "/admin/realms/platform/clients/declarest-cli/resource" {
		t.Fatalf("expected create fallback path to use logical path, got %q", spec.Path)
	}
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
