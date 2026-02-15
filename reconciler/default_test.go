package reconciler

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

func TestDefaultReconcilerDelegatesRepositoryMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepositoryManager{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
		statusValue: repository.SyncReport{State: repository.SyncStateNoRemote},
	}

	reconciler := &DefaultReconciler{
		RepositoryManager: fakeRepo,
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

	status, err := reconciler.RepoStatus(context.Background())
	if err != nil {
		t.Fatalf("RepoStatus returned error: %v", err)
	}
	if status.State != repository.SyncStateNoRemote {
		t.Fatalf("unexpected status state: %q", status.State)
	}

	if !fakeRepo.deletePolicy.Recursive {
		t.Fatal("expected delete policy recursion to be mapped")
	}
	if !fakeRepo.listPolicy.Recursive {
		t.Fatal("expected list policy recursion to be mapped")
	}
}

func TestDefaultReconcilerRequiresRepositoryManager(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultReconciler{}

	_, err := reconciler.RepoStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.ValidationError)
}

type fakeRepositoryManager struct {
	getValue    resource.Value
	getErr      error
	listValue   []resource.Resource
	statusValue repository.SyncReport

	savedPath  string
	savedValue resource.Value

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func (f *fakeRepositoryManager) Save(_ context.Context, logicalPath string, value resource.Value) error {
	f.savedPath = logicalPath
	f.savedValue = value
	return nil
}

func (f *fakeRepositoryManager) Get(context.Context, string) (resource.Value, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeRepositoryManager) Delete(_ context.Context, _ string, policy repository.DeletePolicy) error {
	f.deletePolicy = policy
	return nil
}

func (f *fakeRepositoryManager) List(_ context.Context, _ string, policy repository.ListPolicy) ([]resource.Resource, error) {
	f.listPolicy = policy
	return f.listValue, nil
}

func (f *fakeRepositoryManager) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeRepositoryManager) Move(context.Context, string, string) error   { return nil }
func (f *fakeRepositoryManager) Init(context.Context) error                   { return nil }
func (f *fakeRepositoryManager) Refresh(context.Context) error                { return nil }
func (f *fakeRepositoryManager) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (f *fakeRepositoryManager) Check(context.Context) error { return nil }
func (f *fakeRepositoryManager) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (f *fakeRepositoryManager) SyncStatus(context.Context) (repository.SyncReport, error) {
	return f.statusValue, nil
}

type fakeMetadataService struct {
	resolveValue metadatadomain.ResourceMetadata
	resolveErr   error
}

func (f *fakeMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

func (f *fakeMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (f *fakeMetadataService) Unset(context.Context, string) error {
	return nil
}

func (f *fakeMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	if f.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, f.resolveErr
	}
	return f.resolveValue, nil
}

func (f *fakeMetadataService) RenderOperationSpec(
	ctx context.Context,
	_ string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.ResolveOperationSpec(ctx, f.resolveValue, operation, value)
}

func (f *fakeMetadataService) Infer(context.Context, string, metadatadomain.InferenceRequest) (metadatadomain.ResourceMetadata, error) {
	return f.resolveValue, nil
}

type fakeServerManager struct {
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

func (f *fakeServerManager) Get(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.getCalled = true
	f.lastResource = resourceInfo
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getValue, nil
}

func (f *fakeServerManager) Create(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.createCalled = true
	f.lastResource = resourceInfo
	return f.createValue, nil
}

func (f *fakeServerManager) Update(_ context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	f.updateCalled = true
	f.lastResource = resourceInfo
	return f.updateValue, nil
}

func (f *fakeServerManager) Delete(context.Context, resource.Resource) error {
	return nil
}

func (f *fakeServerManager) List(context.Context, string, metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
	f.listCalled = true
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listValue, nil
}

func (f *fakeServerManager) Exists(context.Context, resource.Resource) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.existsValue, nil
}

func (f *fakeServerManager) GetOpenAPISpec(context.Context) (resource.Value, error) {
	return nil, nil
}

func (f *fakeServerManager) BuildRequestFromMetadata(context.Context, resource.Resource, metadatadomain.Operation) (metadatadomain.OperationSpec, error) {
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

func TestDefaultReconcilerApplyUsesSecretsAndPersistsMaskedPayload(t *testing.T) {
	t.Parallel()

	repo := &fakeRepositoryManager{
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
	metadataService := &fakeMetadataService{resolveValue: md}

	serverManager := &fakeServerManager{
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

	reconciler := &DefaultReconciler{
		RepositoryManager: repo,
		MetadataService:   metadataService,
		ServerManager:     serverManager,
		SecretsProvider:   secretProvider,
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

func TestDefaultReconcilerDiffUsesFallbackAndCompareSuppressRules(t *testing.T) {
	t.Parallel()

	repo := &fakeRepositoryManager{
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

	metadataService := &fakeMetadataService{resolveValue: md}
	serverManager := &fakeServerManager{
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

	reconciler := &DefaultReconciler{
		RepositoryManager: repo,
		MetadataService:   metadataService,
		ServerManager:     serverManager,
		SecretsProvider:   secretProvider,
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

func TestDefaultReconcilerDiffReturnsConflictOnAmbiguousFallback(t *testing.T) {
	t.Parallel()

	repo := &fakeRepositoryManager{
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

	metadataService := &fakeMetadataService{resolveValue: md}
	serverManager := &fakeServerManager{
		getErr: faults.NewTypedError(faults.NotFoundError, "resource not found", nil),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme-1", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
			{LogicalPath: "/customers/acme-2", LocalAlias: "acme", RemoteID: "42", Payload: map[string]any{"id": "42"}},
		},
	}

	reconciler := &DefaultReconciler{
		RepositoryManager: repo,
		MetadataService:   metadataService,
		ServerManager:     serverManager,
	}

	_, err := reconciler.Diff(context.Background(), "/customers/acme")
	assertTypedCategory(t, err, faults.ConflictError)
}

func TestDefaultReconcilerListRemoteSortsDeterministically(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultReconciler{
		RepositoryManager: &fakeRepositoryManager{},
		MetadataService:   &fakeMetadataService{resolveValue: metadatadomain.ResourceMetadata{}},
		ServerManager: &fakeServerManager{
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

func TestDefaultReconcilerTemplateReturnsNormalizedPayload(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultReconciler{
		RepositoryManager: &fakeRepositoryManager{},
		MetadataService: &fakeMetadataService{
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
