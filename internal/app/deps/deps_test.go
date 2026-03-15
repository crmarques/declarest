// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deps

import (
	"context"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestRequireHelpers(t *testing.T) {
	t.Parallel()

	orchestratorService := &noopOrchestrator{}
	contextService := &noopContextService{}
	repositoryStore := &noopRepositoryStore{}
	metadataService := &noopMetadataService{}
	secretProvider := &noopSecretProvider{}

	tests := []struct {
		name     string
		call     func(Dependencies) error
		deps     Dependencies
		category faults.ErrorCategory
	}{
		{
			name: "require orchestrator rejects nil",
			call: func(deps Dependencies) error {
				_, err := RequireOrchestrator(deps)
				return err
			},
			deps:     Dependencies{},
			category: faults.ValidationError,
		},
		{
			name: "require orchestrator accepts value",
			call: func(deps Dependencies) error {
				got, err := RequireOrchestrator(deps)
				if err != nil {
					return err
				}
				if got != orchestratorService {
					t.Fatalf("unexpected orchestrator instance: %#v", got)
				}
				return nil
			},
			deps: Dependencies{Orchestrator: orchestratorService},
		},
		{
			name: "require contexts rejects nil",
			call: func(deps Dependencies) error {
				_, err := RequireContexts(deps)
				return err
			},
			deps:     Dependencies{},
			category: faults.ValidationError,
		},
		{
			name: "require contexts accepts value",
			call: func(deps Dependencies) error {
				got, err := RequireContexts(deps)
				if err != nil {
					return err
				}
				if got != contextService {
					t.Fatalf("unexpected context service instance: %#v", got)
				}
				return nil
			},
			deps: Dependencies{Contexts: contextService},
		},
		{
			name: "require resource store rejects nil",
			call: func(deps Dependencies) error {
				_, err := RequireResourceStore(deps)
				return err
			},
			deps:     Dependencies{},
			category: faults.ValidationError,
		},
		{
			name: "require resource store accepts value",
			call: func(deps Dependencies) error {
				got, err := RequireResourceStore(deps)
				if err != nil {
					return err
				}
				if got != repositoryStore {
					t.Fatalf("unexpected repository store instance: %#v", got)
				}
				return nil
			},
			deps: Dependencies{Services: &noopServiceAccessor{store: repositoryStore}},
		},
		{
			name: "require metadata service rejects nil",
			call: func(deps Dependencies) error {
				_, err := RequireMetadataService(deps)
				return err
			},
			deps:     Dependencies{},
			category: faults.ValidationError,
		},
		{
			name: "require metadata service accepts value",
			call: func(deps Dependencies) error {
				got, err := RequireMetadataService(deps)
				if err != nil {
					return err
				}
				if got != metadataService {
					t.Fatalf("unexpected metadata service instance: %#v", got)
				}
				return nil
			},
			deps: Dependencies{Services: &noopServiceAccessor{metadata: metadataService}},
		},
		{
			name: "require secret provider rejects nil",
			call: func(deps Dependencies) error {
				_, err := RequireSecretProvider(deps)
				return err
			},
			deps:     Dependencies{},
			category: faults.ValidationError,
		},
		{
			name: "require secret provider accepts value",
			call: func(deps Dependencies) error {
				got, err := RequireSecretProvider(deps)
				if err != nil {
					return err
				}
				if got != secretProvider {
					t.Fatalf("unexpected secret provider instance: %#v", got)
				}
				return nil
			},
			deps: Dependencies{Services: &noopServiceAccessor{secrets: secretProvider}},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.call(tc.deps)
			if tc.category == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !faults.IsCategory(err, tc.category) {
				t.Fatalf("expected %s, got %v", tc.category, err)
			}
		})
	}
}

type noopOrchestrator struct{}

func (n *noopOrchestrator) GetLocal(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}

func (n *noopOrchestrator) ListLocal(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}

func (n *noopOrchestrator) GetRemote(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}

func (n *noopOrchestrator) ListRemote(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}

func (n *noopOrchestrator) GetOpenAPISpec(context.Context) (resource.Content, error) {
	return resource.Content{}, nil
}

func (n *noopOrchestrator) Request(context.Context, managedserverdomain.RequestSpec) (resource.Content, error) {
	return resource.Content{}, nil
}

func (n *noopOrchestrator) Save(context.Context, string, resource.Content) error {
	return nil
}

func (n *noopOrchestrator) Apply(context.Context, string, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (n *noopOrchestrator) ApplyWithContent(context.Context, string, resource.Content, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (n *noopOrchestrator) Create(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (n *noopOrchestrator) Update(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (n *noopOrchestrator) Delete(context.Context, string, orchestratordomain.DeletePolicy) error {
	return nil
}

func (n *noopOrchestrator) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}

func (n *noopOrchestrator) Template(context.Context, string, resource.Content) (resource.Content, error) {
	return resource.Content{}, nil
}

type noopContextService struct{}

func (n *noopContextService) Create(context.Context, configdomain.Context) error { return nil }
func (n *noopContextService) Update(context.Context, configdomain.Context) error { return nil }
func (n *noopContextService) Delete(context.Context, string) error               { return nil }
func (n *noopContextService) Rename(context.Context, string, string) error       { return nil }
func (n *noopContextService) List(context.Context) ([]configdomain.Context, error) {
	return nil, nil
}
func (n *noopContextService) SetCurrent(context.Context, string) error { return nil }
func (n *noopContextService) GetCurrent(context.Context) (configdomain.Context, error) {
	return configdomain.Context{}, nil
}
func (n *noopContextService) ResolveContext(context.Context, configdomain.ContextSelection) (configdomain.Context, error) {
	return configdomain.Context{}, nil
}
func (n *noopContextService) Validate(context.Context, configdomain.Context) error { return nil }

type noopRepositoryStore struct{}

func (n *noopRepositoryStore) Save(context.Context, string, resource.Content) error { return nil }
func (n *noopRepositoryStore) Get(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}
func (n *noopRepositoryStore) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (n *noopRepositoryStore) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (n *noopRepositoryStore) Exists(context.Context, string) (bool, error) { return false, nil }

type noopServiceAccessor struct {
	store    repository.ResourceStore
	metadata metadatadomain.MetadataService
	secrets  secretdomain.SecretProvider
}

func (n *noopServiceAccessor) RepositoryStore() repository.ResourceStore { return n.store }
func (n *noopServiceAccessor) RepositorySync() repository.RepositorySync { return nil }
func (n *noopServiceAccessor) MetadataService() metadatadomain.MetadataService {
	return n.metadata
}
func (n *noopServiceAccessor) SecretProvider() secretdomain.SecretProvider { return n.secrets }
func (n *noopServiceAccessor) ManagedServerClient() managedserverdomain.ManagedServerClient {
	return nil
}

type noopMetadataService struct{}

func (n *noopMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}
func (n *noopMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}
func (n *noopMetadataService) Unset(context.Context, string) error { return nil }
func (n *noopMetadataService) ResolveForPath(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}
func (n *noopMetadataService) RenderOperationSpec(context.Context, string, metadatadomain.Operation, any) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}

type noopSecretProvider struct{}

func (n *noopSecretProvider) Init(context.Context) error { return nil }
func (n *noopSecretProvider) Store(context.Context, string, string) error {
	return nil
}
func (n *noopSecretProvider) Get(context.Context, string) (string, error) { return "", nil }
func (n *noopSecretProvider) Delete(context.Context, string) error        { return nil }
func (n *noopSecretProvider) List(context.Context) ([]string, error)      { return nil, nil }
func (n *noopSecretProvider) MaskPayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (n *noopSecretProvider) ResolvePayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (n *noopSecretProvider) NormalizeSecretPlaceholders(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (n *noopSecretProvider) DetectSecretCandidates(context.Context, resource.Value) ([]string, error) {
	return nil, nil
}
