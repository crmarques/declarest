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

package fsmetadata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func TestLayeredMetadataServiceResolveForPathAppliesRepoLocalPrecedence(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	shared := NewFSMetadataService(sharedDir)
	local := NewFSMetadataService(localDir)
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteLocal)
	ctx := context.Background()

	mustSetMetadata(t, shared, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		ID:     "{{/sharedId}}",
		Format: "yaml",
	})
	mustSetMetadata(t, local, ctx, "/customers/_", metadatadomain.ResourceMetadata{
		Alias:  "{{/name}}",
		Format: "json",
	})
	mustSetMetadata(t, local, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		RequiredAttributes: []string{"/name"},
	})

	resolved, err := service.ResolveForPath(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("ResolveForPath returned error: %v", err)
	}
	if resolved.ID != "{{/sharedId}}" {
		t.Fatalf("expected shared id to remain, got %q", resolved.ID)
	}
	if resolved.Alias != "{{/name}}" {
		t.Fatalf("expected repo-local alias override, got %q", resolved.Alias)
	}
	if resolved.Format != "json" {
		t.Fatalf("expected repo-local format override, got %q", resolved.Format)
	}
	if !reflect.DeepEqual(resolved.RequiredAttributes, []string{"/name"}) {
		t.Fatalf("expected repo-local resource required attributes, got %#v", resolved.RequiredAttributes)
	}
}

func TestLayeredMetadataServiceGetSetUnsetTargetsLocalOverlay(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	shared := NewFSMetadataService(sharedDir)
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteLocal)
	ctx := context.Background()

	mustSetMetadata(t, shared, ctx, "/customers/acme", metadatadomain.ResourceMetadata{
		ID: "{{/id}}",
	})

	if _, err := service.Get(ctx, "/customers/acme"); !faults.IsCategory(err, faults.NotFoundError) {
		t.Fatalf("expected Get to read only the writable local overlay, got %v", err)
	}

	localMetadata := metadatadomain.ResourceMetadata{Alias: "{{/name}}"}
	if err := service.Set(ctx, "/customers/acme", localMetadata); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error after Set: %v", err)
	}
	if !reflect.DeepEqual(got, localMetadata) {
		t.Fatalf("expected local metadata %#v, got %#v", localMetadata, got)
	}

	localPath := filepath.Join(localDir, "customers", "acme", "metadata.yaml")
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected local overlay file %q, got %v", localPath, err)
	}
	sharedPath := filepath.Join(sharedDir, "customers", "acme", "metadata.yaml")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared metadata file %q to remain unchanged, got %v", sharedPath, err)
	}

	if err := service.Unset(ctx, "/customers/acme"); err != nil {
		t.Fatalf("Unset returned error: %v", err)
	}
	if _, err := service.Get(ctx, "/customers/acme"); !faults.IsCategory(err, faults.NotFoundError) {
		t.Fatalf("expected Get to return not found after local unset, got %v", err)
	}
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared metadata file %q to remain after local unset, got %v", sharedPath, err)
	}
}

func TestLayeredMetadataServiceGetSetUnsetTargetsSharedSource(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteShared)
	ctx := context.Background()

	sharedMetadata := metadatadomain.ResourceMetadata{Alias: "{{/name}}"}
	if err := service.Set(ctx, "/customers/acme", sharedMetadata); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := service.Get(ctx, "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error after Set: %v", err)
	}
	if !reflect.DeepEqual(got, sharedMetadata) {
		t.Fatalf("expected shared metadata %#v, got %#v", sharedMetadata, got)
	}

	sharedPath := filepath.Join(sharedDir, "customers", "acme", "metadata.yaml")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared metadata file %q, got %v", sharedPath, err)
	}
	localPath := filepath.Join(localDir, "customers", "acme", "metadata.yaml")
	if _, err := os.Stat(localPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected local overlay file %q to remain absent, got %v", localPath, err)
	}

	if err := service.Unset(ctx, "/customers/acme"); err != nil {
		t.Fatalf("Unset returned error: %v", err)
	}
	if _, err := service.Get(ctx, "/customers/acme"); !faults.IsCategory(err, faults.NotFoundError) {
		t.Fatalf("expected Get to return not found after shared unset, got %v", err)
	}
	if _, err := os.Stat(sharedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected shared metadata file %q to be removed, got %v", sharedPath, err)
	}
}

func TestLayeredMetadataServiceCollectionResolversUnionSources(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	shared := NewFSMetadataService(sharedDir)
	local := NewFSMetadataService(localDir)
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteLocal)
	ctx := context.Background()

	mustSetMetadata(t, shared, ctx, "/admin/realms/_/user-registry/_/mappers/_", metadatadomain.ResourceMetadata{
		ID: "{{/id}}",
	})
	mustSetMetadata(t, local, ctx, "/admin/realms/_/user-registry/_/policies/_", metadatadomain.ResourceMetadata{
		ID: "{{/id}}",
	})
	mustSetMetadata(t, shared, ctx, "/admin/realms/_/authentication/flows/_/executions/_", metadatadomain.ResourceMetadata{
		ID: "{{/id}}",
	})

	children, err := service.ResolveCollectionChildren(ctx, "/admin/realms/master/user-registry/AD PRD")
	if err != nil {
		t.Fatalf("ResolveCollectionChildren returned error: %v", err)
	}
	if !reflect.DeepEqual(children, []string{"mappers", "policies"}) {
		t.Fatalf("expected unioned child branches, got %#v", children)
	}

	ok, err := service.HasCollectionWildcardChild(ctx, "/admin/realms/master/authentication/flows/test/executions")
	if err != nil {
		t.Fatalf("HasCollectionWildcardChild returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected wildcard child resolution across layered sources")
	}
}

func TestLayeredMetadataServiceDefaultsArtifactsWriteToLocalOverlay(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	shared := NewFSMetadataService(sharedDir)
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteLocal)
	ctx := context.Background()

	if err := shared.WriteDefaultsArtifact(ctx, "/customers/", "defaults.yaml", resource.Content{
		Value: map[string]any{"shared": true},
	}); err != nil {
		t.Fatalf("shared WriteDefaultsArtifact returned error: %v", err)
	}

	if _, err := service.ReadDefaultsArtifact(ctx, "/customers/", "defaults.yaml"); !faults.IsCategory(err, faults.NotFoundError) {
		t.Fatalf("expected layered defaults read to target local overlay only, got %v", err)
	}

	if err := service.WriteDefaultsArtifact(ctx, "/customers/", "defaults.yaml", resource.Content{
		Value: map[string]any{"local": true},
	}); err != nil {
		t.Fatalf("WriteDefaultsArtifact returned error: %v", err)
	}

	content, err := service.ReadDefaultsArtifact(ctx, "/customers/", "defaults.yaml")
	if err != nil {
		t.Fatalf("ReadDefaultsArtifact returned error after local write: %v", err)
	}
	value, ok := content.Value.(map[string]any)
	if !ok || value["local"] != true {
		t.Fatalf("expected local defaults artifact payload, got %#v", content.Value)
	}

	localPath := filepath.Join(localDir, "customers", "_", "defaults.yaml")
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected local defaults artifact %q, got %v", localPath, err)
	}
	sharedPath := filepath.Join(sharedDir, "customers", "_", "defaults.yaml")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared defaults artifact %q to remain, got %v", sharedPath, err)
	}
}

func TestLayeredMetadataServiceDefaultsArtifactsWriteToSharedSource(t *testing.T) {
	t.Parallel()

	sharedDir := t.TempDir()
	localDir := t.TempDir()
	service := NewLayeredFSMetadataService(sharedDir, localDir, LayeredMetadataWriteShared)
	ctx := context.Background()

	if err := service.WriteDefaultsArtifact(ctx, "/customers/_", "defaults.yaml", resource.Content{
		Value: map[string]any{"shared": true},
	}); err != nil {
		t.Fatalf("WriteDefaultsArtifact returned error: %v", err)
	}

	content, err := service.ReadDefaultsArtifact(ctx, "/customers/_", "defaults.yaml")
	if err != nil {
		t.Fatalf("ReadDefaultsArtifact returned error after shared write: %v", err)
	}
	value, ok := content.Value.(map[string]any)
	if !ok || value["shared"] != true {
		t.Fatalf("expected shared defaults artifact payload, got %#v", content.Value)
	}

	sharedPath := filepath.Join(sharedDir, "customers", "_", "defaults.yaml")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared defaults artifact %q, got %v", sharedPath, err)
	}
	localPath := filepath.Join(localDir, "customers", "_", "defaults.yaml")
	if _, err := os.Stat(localPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected local defaults artifact %q to remain absent, got %v", localPath, err)
	}
}

func TestLayeredMetadataServiceNilConfigurationReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := NewLayeredFSMetadataService("", "", LayeredMetadataWriteLocal)
	_, err := service.Get(context.Background(), "/customers/acme")
	if err == nil {
		t.Fatal("expected Get error for nil configuration")
	}
	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) || typedErr.Category != faults.NotFoundError {
		t.Fatalf("expected typed not found error, got %v", err)
	}
}
