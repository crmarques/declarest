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

package managedservice

import (
	"context"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

type RequestSpec struct {
	Method      string
	Path        string
	Query       map[string]string
	Headers     map[string]string
	Accept      string
	ContentType string
	Body        resource.Content
}

type ManagedServiceClient interface {
	Get(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Create(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Update(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Content, error)
	Delete(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) error
	List(ctx context.Context, collectionPath string, metadata metadata.ResourceMetadata) ([]resource.Resource, error)
	Exists(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (bool, error)
	Request(ctx context.Context, spec RequestSpec) (resource.Content, error)
	GetOpenAPISpec(ctx context.Context) (resource.Content, error)
}

// AccessTokenProvider is an optional managed-service capability used by CLI
// inspection commands to retrieve an OAuth2 access token when supported.
type AccessTokenProvider interface {
	GetAccessToken(ctx context.Context) (string, error)
}
