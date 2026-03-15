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

	"github.com/crmarques/declarest/resource"
)

type MetadataStore interface {
	Get(ctx context.Context, logicalPath string) (ResourceMetadata, error)
	Set(ctx context.Context, logicalPath string, metadata ResourceMetadata) error
	Unset(ctx context.Context, logicalPath string) error
}

type MetadataResolver interface {
	ResolveForPath(ctx context.Context, logicalPath string) (ResourceMetadata, error)
}

type OperationSpecRenderer interface {
	RenderOperationSpec(ctx context.Context, logicalPath string, operation Operation, value any) (OperationSpec, error)
}

type ResourceOperationSpecInput struct {
	LogicalPath       string
	CollectionPath    string
	LocalAlias        string
	RemoteID          string
	PayloadDescriptor resource.PayloadDescriptor
	Metadata          ResourceMetadata
	Payload           any
}

// ResourceOperationSpecRenderer is an optional metadata capability used by
// orchestrator/server adapters to render operation specs with fully derived
// resource identity/context.
type ResourceOperationSpecRenderer interface {
	RenderOperationSpecForResource(ctx context.Context, resource ResourceOperationSpecInput, operation Operation) (OperationSpec, error)
}

type MetadataService interface {
	MetadataStore
	MetadataResolver
	OperationSpecRenderer
}

// DefaultsArtifactStore is an optional metadata capability used by defaults
// workflows that persist deterministic file-backed defaults artifacts next to
// metadata selector files.
type DefaultsArtifactStore interface {
	ReadDefaultsArtifact(ctx context.Context, logicalPath string, file string) (resource.Content, error)
	WriteDefaultsArtifact(ctx context.Context, logicalPath string, file string, content resource.Content) error
	DeleteDefaultsArtifact(ctx context.Context, logicalPath string, file string) error
}

// CollectionChildrenResolver is an optional metadata capability used by path
// completion to surface child selectors that exist only in metadata templates.
type CollectionChildrenResolver interface {
	ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error)
}

// CollectionWildcardResolver is an optional metadata capability used by
// fallback helpers to know when wildcard selectors are available under a
// collection branch.
type CollectionWildcardResolver interface {
	HasCollectionWildcardChild(ctx context.Context, logicalPath string) (bool, error)
}
