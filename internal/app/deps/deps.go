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
	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
	Contexts     configdomain.ContextService
	Repository   repository.ResourceStore
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
	Services     orchestratordomain.ServiceAccessor
}

func (deps Dependencies) ResourceStore() repository.ResourceStore {
	if deps.Repository != nil {
		return deps.Repository
	}
	if deps.Services == nil {
		return nil
	}
	return deps.Services.RepositoryStore()
}

func (deps Dependencies) MetadataService() metadatadomain.MetadataService {
	if deps.Metadata != nil {
		return deps.Metadata
	}
	if deps.Services == nil {
		return nil
	}
	return deps.Services.MetadataService()
}

func (deps Dependencies) SecretProvider() secretdomain.SecretProvider {
	if deps.Secrets != nil {
		return deps.Secrets
	}
	if deps.Services == nil {
		return nil
	}
	return deps.Services.SecretProvider()
}

func RequireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, faults.NewValidationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func RequireContexts(deps Dependencies) (configdomain.ContextService, error) {
	if deps.Contexts == nil {
		return nil, faults.NewValidationError("context service is not configured", nil)
	}
	return deps.Contexts, nil
}

func RequireResourceStore(deps Dependencies) (repository.ResourceStore, error) {
	store := deps.ResourceStore()
	if store == nil {
		return nil, faults.NewValidationError("resource repository is not configured", nil)
	}
	return store, nil
}

func RequireMetadataService(deps Dependencies) (metadatadomain.MetadataService, error) {
	service := deps.MetadataService()
	if service == nil {
		return nil, faults.NewValidationError("metadata service is not configured", nil)
	}
	return service, nil
}

func RequireSecretProvider(deps Dependencies) (secretdomain.SecretProvider, error) {
	provider := deps.SecretProvider()
	if provider == nil {
		return nil, faults.NewValidationError("secret provider is not configured", nil)
	}
	return provider, nil
}
