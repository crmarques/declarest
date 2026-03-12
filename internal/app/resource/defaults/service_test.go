package defaults

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestGetReturnsEmptyObjectWhenDefaultsSidecarIsMissing(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Get(context.Background(), deps, "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	want := map[string]any{}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected defaults payload: got %#v want %#v", result.Content.Value, want)
	}
	if result.Content.Descriptor.PayloadType != resource.PayloadTypeJSON {
		t.Fatalf("expected json defaults descriptor, got %#v", result.Content.Descriptor)
	}
}

func TestInferFromRepositoryUsesCommonSiblingValues(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"labels": map[string]any{
			"team": "platform",
		},
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected inferred defaults: got %#v want %#v", result.Content.Value, want)
	}
}

func TestInferFromManagedServerCreatesAndDeletesTemporaryResources(t *testing.T) {
	t.Parallel()

	deps := testDefaultsDeps()
	orch := deps.Orchestrator.(*fakeDefaultsOrchestrator)
	deps.Metadata = &fakeDefaultsMetadata{
		items: map[string]metadata.ResourceMetadata{
			"/customers/acme": {
				ID:    "{{/id}}",
				Alias: "{{/name}}",
			},
		},
	}

	result, err := Infer(context.Background(), deps, "/customers/acme", InferRequest{ManagedServer: true})
	if err != nil {
		t.Fatalf("Infer returned error: %v", err)
	}

	want := map[string]any{
		"status": "active",
	}
	if !reflect.DeepEqual(result.Content.Value, want) {
		t.Fatalf("unexpected managed-server defaults: got %#v want %#v", result.Content.Value, want)
	}

	if len(orch.createCalls) != 2 {
		t.Fatalf("expected two temporary creates, got %#v", orch.createCalls)
	}
	if len(orch.deleteCalls) != 2 {
		t.Fatalf("expected two cleanup deletes, got %#v", orch.deleteCalls)
	}
	if orch.deleteCalls[0] != orch.createCalls[1].logicalPath || orch.deleteCalls[1] != orch.createCalls[0].logicalPath {
		t.Fatalf("expected cleanup deletes in reverse order, got creates=%#v deletes=%#v", orch.createCalls, orch.deleteCalls)
	}
}

type fakeDefaultsOrchestrator struct {
	orchestratordomain.Orchestrator
	localContent map[string]resource.Content
	createCalls  []savedResource
	deleteCalls  []string
}

type savedResource struct {
	logicalPath string
	content     resource.Content
}

func (f *fakeDefaultsOrchestrator) ResolveLocalResource(_ context.Context, logicalPath string) (resource.Resource, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Resource{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return resource.Resource{
		LogicalPath:       logicalPath,
		CollectionPath:    collectionPathFor(logicalPath),
		Payload:           content.Value,
		PayloadDescriptor: content.Descriptor,
	}, nil
}

func (f *fakeDefaultsOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.localContent[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
	}
	return content, nil
}

func (f *fakeDefaultsOrchestrator) ListLocal(_ context.Context, logicalPath string, _ orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	items := make([]resource.Resource, 0, len(f.localContent))
	for candidate := range f.localContent {
		if collectionPathFor(candidate) != logicalPath {
			continue
		}
		items = append(items, resource.Resource{LogicalPath: candidate})
	}
	return items, nil
}

func (f *fakeDefaultsOrchestrator) Create(_ context.Context, logicalPath string, content resource.Content) (resource.Resource, error) {
	f.createCalls = append(f.createCalls, savedResource{logicalPath: logicalPath, content: content})
	return resource.Resource{LogicalPath: logicalPath}, nil
}

func (f *fakeDefaultsOrchestrator) GetRemote(_ context.Context, logicalPath string) (resource.Content, error) {
	for _, item := range f.createCalls {
		if item.logicalPath != logicalPath {
			continue
		}
		payload := resource.DeepCopyValue(item.content.Value).(map[string]any)
		payload["status"] = "active"
		payload["id"] = path.Base(logicalPath)
		payload["name"] = path.Base(logicalPath)
		return resource.Content{
			Value:      payload,
			Descriptor: item.content.Descriptor,
		}, nil
	}
	return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "not found", nil)
}

func (f *fakeDefaultsOrchestrator) Delete(_ context.Context, logicalPath string, _ orchestratordomain.DeletePolicy) error {
	f.deleteCalls = append(f.deleteCalls, logicalPath)
	return nil
}

type fakeDefaultsRepository struct {
	repository.ResourceStore
	defaults map[string]resource.Content
}

func (f *fakeDefaultsRepository) Save(context.Context, string, resource.Content) error { return nil }
func (f *fakeDefaultsRepository) Get(_ context.Context, logicalPath string) (resource.Content, error) {
	return resource.Content{}, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("resource %q not found", logicalPath), nil)
}
func (f *fakeDefaultsRepository) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (f *fakeDefaultsRepository) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (f *fakeDefaultsRepository) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeDefaultsRepository) GetDefaults(_ context.Context, logicalPath string) (resource.Content, error) {
	content, found := f.defaults[logicalPath]
	if !found {
		return resource.Content{}, faults.NewTypedError(faults.NotFoundError, "defaults not found", nil)
	}
	return content, nil
}
func (f *fakeDefaultsRepository) SaveDefaults(_ context.Context, logicalPath string, content resource.Content) error {
	if f.defaults == nil {
		f.defaults = map[string]resource.Content{}
	}
	f.defaults[logicalPath] = content
	return nil
}

type fakeDefaultsMetadata struct {
	metadata.MetadataService
	items map[string]metadata.ResourceMetadata
}

func (f *fakeDefaultsMetadata) ResolveForPath(_ context.Context, logicalPath string) (metadata.ResourceMetadata, error) {
	if item, found := f.items[logicalPath]; found {
		return item, nil
	}
	return metadata.ResourceMetadata{}, nil
}

type fakeDefaultsServiceAccessor struct {
	store    repository.ResourceStore
	metadata metadata.MetadataService
	secrets  secretdomain.SecretProvider
}

func (f *fakeDefaultsServiceAccessor) RepositoryStore() repository.ResourceStore   { return f.store }
func (f *fakeDefaultsServiceAccessor) RepositorySync() repository.RepositorySync   { return nil }
func (f *fakeDefaultsServiceAccessor) MetadataService() metadata.MetadataService   { return f.metadata }
func (f *fakeDefaultsServiceAccessor) SecretProvider() secretdomain.SecretProvider { return f.secrets }
func (f *fakeDefaultsServiceAccessor) ManagedServerClient() managedserver.ManagedServerClient {
	return nil
}

func testDefaultsDeps() appdeps.Dependencies {
	orch := &fakeDefaultsOrchestrator{
		localContent: map[string]resource.Content{
			"/customers/acme": {
				Value: map[string]any{
					"id":     "acme",
					"name":   "acme",
					"status": "custom-a",
					"labels": map[string]any{"team": "platform"},
				},
				Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
			},
			"/customers/beta": {
				Value: map[string]any{
					"id":     "beta",
					"name":   "beta",
					"status": "custom-b",
					"labels": map[string]any{"team": "platform"},
				},
				Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
			},
		},
	}
	repo := &fakeDefaultsRepository{defaults: map[string]resource.Content{}}
	md := &fakeDefaultsMetadata{items: map[string]metadata.ResourceMetadata{}}

	return appdeps.Dependencies{
		Orchestrator: orch,
		Repository:   repo,
		Metadata:     md,
		Services: &fakeDefaultsServiceAccessor{
			store:    repo,
			metadata: md,
		},
	}
}
