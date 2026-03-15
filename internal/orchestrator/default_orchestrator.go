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

package orchestrator

import (
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

var _ orchestrator.Orchestrator = (*Orchestrator)(nil)

type Orchestrator struct {
	repository repository.ResourceStore
	metadata   metadata.MetadataService
	server     managedserver.ManagedServerClient
	secrets    secrets.SecretProvider
}

// Option configures optional orchestrator settings.
type Option func(*Orchestrator)

func WithDefaultFormat(format string) Option {
	_ = format
	return func(*Orchestrator) {}
}

func New(
	repo repository.ResourceStore,
	meta metadata.MetadataService,
	srv managedserver.ManagedServerClient,
	sec secrets.SecretProvider,
	opts ...Option,
) *Orchestrator {
	o := &Orchestrator{
		repository: repo,
		metadata:   meta,
		server:     srv,
		secrets:    sec,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (r *Orchestrator) RepositoryStore() repository.ResourceStore {
	if r == nil {
		return nil
	}
	return r.repository
}

func (r *Orchestrator) RepositorySync() repository.RepositorySync {
	if r == nil || r.repository == nil {
		return nil
	}
	if sync, ok := r.repository.(repository.RepositorySync); ok {
		return sync
	}
	return nil
}

func (r *Orchestrator) MetadataService() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.metadata
}

func (r *Orchestrator) ManagedServerClient() managedserver.ManagedServerClient {
	if r == nil {
		return nil
	}
	return r.server
}

func (r *Orchestrator) SecretProvider() secrets.SecretProvider {
	if r == nil {
		return nil
	}
	return r.secrets
}

func (r *Orchestrator) applyDefaultFormat(content resource.Content, md metadata.ResourceMetadata) resource.Content {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return content
	}

	format := metadata.NormalizeResourceFormat(md.Format)
	if format == "" || metadata.ResourceFormatAllowsMixedItems(format) {
		return content
	}

	content.Descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: format})
	return content
}
