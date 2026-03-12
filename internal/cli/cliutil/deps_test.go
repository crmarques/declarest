package cliutil

import (
	"context"
	"testing"

	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretsdomain "github.com/crmarques/declarest/secrets"
)

func TestAppDependencies(t *testing.T) {
	t.Parallel()

	store := &stubResourceStore{}
	metadataService := &stubMetadataService{}
	secretProvider := &stubSecretProvider{}

	deps := AppDependencies(CommandDependencies{
		Services: &testServiceAccessor{
			store:    store,
			metadata: metadataService,
			secrets:  secretProvider,
		},
	})

	if deps.Repository != store {
		t.Fatalf("expected repository dependency to be copied")
	}
	if deps.Metadata != metadataService {
		t.Fatalf("expected metadata dependency to be copied")
	}
	if deps.Secrets != secretProvider {
		t.Fatalf("expected secrets dependency to be copied")
	}
}

type testServiceAccessor struct {
	store    repository.ResourceStore
	sync     repository.RepositorySync
	metadata metadatadomain.MetadataService
	secrets  secretsdomain.SecretProvider
	server   managedserverdomain.ManagedServerClient
}

func (a *testServiceAccessor) RepositoryStore() repository.ResourceStore {
	return a.store
}

func (a *testServiceAccessor) RepositorySync() repository.RepositorySync {
	return a.sync
}

func (a *testServiceAccessor) MetadataService() metadatadomain.MetadataService {
	return a.metadata
}

func (a *testServiceAccessor) SecretProvider() secretsdomain.SecretProvider {
	return a.secrets
}

func (a *testServiceAccessor) ManagedServerClient() managedserverdomain.ManagedServerClient {
	return a.server
}

type stubResourceStore struct{}

func (s *stubResourceStore) Save(context.Context, string, resource.Content) error {
	return nil
}

func (s *stubResourceStore) Get(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}

func (s *stubResourceStore) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}

func (s *stubResourceStore) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}

func (s *stubResourceStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

type stubMetadataService struct{}

func (s *stubMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (s *stubMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}

func (s *stubMetadataService) Unset(context.Context, string) error {
	return nil
}

func (s *stubMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

func (s *stubMetadataService) RenderOperationSpec(context.Context, string, metadatadomain.Operation, any) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

type stubSecretProvider struct{}

func (s *stubSecretProvider) Init(context.Context) error {
	return nil
}

func (s *stubSecretProvider) Store(context.Context, string, string) error {
	return nil
}

func (s *stubSecretProvider) Get(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubSecretProvider) Delete(context.Context, string) error {
	return nil
}

func (s *stubSecretProvider) List(context.Context) ([]string, error) {
	return nil, nil
}

func (s *stubSecretProvider) MaskPayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}

func (s *stubSecretProvider) ResolvePayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}

func (s *stubSecretProvider) NormalizeSecretPlaceholders(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}

func (s *stubSecretProvider) DetectSecretCandidates(context.Context, resource.Value) ([]string, error) {
	return nil, nil
}
