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

package metadata

import (
	"context"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func TestResolvedMetadataForGetKeepsResolvedMetadataOnlyOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata metadatadomain.ResourceMetadata
	}{
		{
			name:     "format",
			metadata: metadatadomain.ResourceMetadata{Format: "yaml"},
		},
		{
			name:     "text_format",
			metadata: metadatadomain.ResourceMetadata{Format: "text"},
		},
		{
			name:     "required_attributes",
			metadata: metadatadomain.ResourceMetadata{RequiredAttributes: []string{}},
		},
		{
			name: "externalized_attributes",
			metadata: metadatadomain.ResourceMetadata{
				ExternalizedAttributes: []metadatadomain.ExternalizedAttribute{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &resolvedMetadataForGetService{
				resolved: map[string]metadatadomain.ResourceMetadata{
					"/customers/acme": tt.metadata,
				},
			}
			deps := cliutil.CommandDependencies{
				Services: resolvedMetadataForGetAccessor{metadata: service},
			}

			got, err := resolvedMetadataForGet(context.Background(), deps, service, "/customers/acme")
			if err != nil {
				t.Fatalf("resolvedMetadataForGet returned error: %v", err)
			}
			if !reflect.DeepEqual(tt.metadata, got) {
				t.Fatalf("expected resolved metadata %#v, got %#v", tt.metadata, got)
			}
			if len(service.getCalls) != 0 {
				t.Fatalf("expected Get not to be called, got %#v", service.getCalls)
			}
		})
	}
}

type resolvedMetadataForGetService struct {
	metadatadomain.MetadataService
	resolved map[string]metadatadomain.ResourceMetadata
	explicit map[string]metadatadomain.ResourceMetadata
	getCalls []string
}

func (s *resolvedMetadataForGetService) Get(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	s.getCalls = append(s.getCalls, logicalPath)
	if item, ok := s.explicit[logicalPath]; ok {
		return item, nil
	}
	return metadatadomain.ResourceMetadata{}, faults.NewTypedError(faults.NotFoundError, "metadata not found", nil)
}

func (s *resolvedMetadataForGetService) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	if item, ok := s.resolved[logicalPath]; ok {
		return item, nil
	}
	return metadatadomain.ResourceMetadata{}, nil
}

type resolvedMetadataForGetAccessor struct {
	orchestrator.ServiceAccessor
	metadata metadatadomain.MetadataService
}

func (a resolvedMetadataForGetAccessor) RepositoryStore() repository.ResourceStore { return nil }
func (a resolvedMetadataForGetAccessor) RepositorySync() repository.RepositorySync { return nil }
func (a resolvedMetadataForGetAccessor) MetadataService() metadatadomain.MetadataService {
	return a.metadata
}
func (a resolvedMetadataForGetAccessor) SecretProvider() secretdomain.SecretProvider { return nil }
func (a resolvedMetadataForGetAccessor) ManagedServerClient() managedserverdomain.ManagedServerClient {
	return nil
}
