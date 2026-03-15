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

package controllers

import (
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
)

func TestPopulateMetadataConfigWithBundlePrefersBundleRef(t *testing.T) {
	t.Parallel()

	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle("/tmp/ignored.tar.gz", "keycloak-bundle:0.0.1", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.Bundle != "keycloak-bundle:0.0.1" {
		t.Fatalf("expected metadata bundle ref to be preserved, got %q", ctx.Metadata.Bundle)
	}
	if ctx.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata base-dir to remain empty, got %q", ctx.Metadata.BaseDir)
	}
}

func TestPopulateMetadataConfigStillSupportsDirectoryArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle(dir, "", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.BaseDir != dir {
		t.Fatalf("expected metadata base-dir %q, got %q", dir, ctx.Metadata.BaseDir)
	}
	if ctx.Metadata.Bundle != "" {
		t.Fatalf("expected metadata bundle to remain empty, got %q", ctx.Metadata.Bundle)
	}
}

func TestPopulateMetadataConfigStillSupportsBundleArchives(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "bundle.tar.gz")
	ctx := &config.Context{}
	err := populateMetadataConfigWithBundle(archivePath, "", ctx)
	if err != nil {
		t.Fatalf("populateMetadataConfigWithBundle() unexpected error: %v", err)
	}
	if ctx.Metadata.Bundle != archivePath {
		t.Fatalf("expected metadata bundle archive %q, got %q", archivePath, ctx.Metadata.Bundle)
	}
	if ctx.Metadata.BaseDir != "" {
		t.Fatalf("expected metadata base-dir to remain empty, got %q", ctx.Metadata.BaseDir)
	}
}
