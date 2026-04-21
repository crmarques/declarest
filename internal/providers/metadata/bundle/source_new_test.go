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

package bundlemetadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigMapRefAcceptsCanonicalForm(t *testing.T) {
	source, err := parseBundleSource("configmap://ops/haproxy-bundle/bundle.tar.gz")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if source.kind != sourceKindConfigMap {
		t.Fatalf("expected configmap kind, got %q", source.kind)
	}
	if source.inMemoryKey != "configmap://ops/haproxy-bundle/bundle.tar.gz" {
		t.Fatalf("unexpected in-memory key: %q", source.inMemoryKey)
	}
	if source.cacheDirName == "" {
		t.Fatalf("expected non-empty cacheDirName")
	}
}

func TestParseConfigMapRefRejectsTruncated(t *testing.T) {
	if _, err := parseBundleSource("configmap://only/two"); err == nil {
		t.Fatal("expected error for truncated configmap ref")
	}
}

func TestParseBundleSourceFileURL(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(archive, []byte{0x1f, 0x8b}, 0o600); err != nil {
		t.Fatalf("failed to seed archive: %v", err)
	}
	source, err := parseBundleSource("file://" + archive)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if source.kind != sourceKindLocal {
		t.Fatalf("expected local kind, got %q", source.kind)
	}
	if source.localPath != archive {
		t.Fatalf("expected localPath %q, got %q", archive, source.localPath)
	}
}

func TestResolveBundleConfigMapUsesInMemoryBytes(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: haproxy
version: 0.0.1
description: HAProxy metadata bundle.
declarest:
  metadataRoot: metadata
`,
		"metadata/metadata.json": `{}`,
	})

	ref := "configmap://ops/haproxy-bundle/bundle.tar.gz"
	resolved, err := ResolveBundle(
		context.Background(),
		ref,
		WithInMemoryBundles(map[string][]byte{ref: archive}),
	)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if resolved.Manifest.Name != "haproxy" {
		t.Fatalf("expected bundle name haproxy, got %q", resolved.Manifest.Name)
	}
	if _, err := os.Stat(filepath.Join(resolved.MetadataDir, "metadata.json")); err != nil {
		t.Fatalf("expected metadata tree under resolved directory: %v", err)
	}
}

func TestResolveBundleConfigMapMissingBytesFails(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	if _, err := ResolveBundle(
		context.Background(),
		"configmap://ops/missing/bundle.tar.gz",
	); err == nil {
		t.Fatal("expected error for missing in-memory bundle bytes")
	}
}

func TestResolveBundleDirectoryInPlace(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "bundle.yaml"), []byte(`
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: haproxy
version: 0.0.1
description: HAProxy metadata bundle.
declarest:
  metadataRoot: metadata
`), 0o600); err != nil {
		t.Fatalf("failed to write bundle.yaml: %v", err)
	}
	metadataDir := filepath.Join(sourceDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("failed to create metadata dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metadataDir, "metadata.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), sourceDir)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if resolved.MetadataDir != filepath.Join(sourceDir, "metadata") {
		t.Fatalf("expected in-place metadata dir, got %q", resolved.MetadataDir)
	}

	// Verify the controlled HOME's cache root was not used.
	cacheRoot := filepath.Join(tempHome, defaultBundleCacheDir)
	if _, err := os.Stat(cacheRoot); !os.IsNotExist(err) {
		t.Fatalf("expected no cache root for in-place resolution, got stat err=%v", err)
	}
}

func TestResolveBundleWithCacheRootOverride(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: haproxy
version: 0.0.1
description: HAProxy metadata bundle.
declarest:
  metadataRoot: metadata
`,
		"metadata/metadata.json": `{}`,
	})
	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	customCache := filepath.Join(t.TempDir(), "bundle-cache")
	resolved, err := ResolveBundle(
		context.Background(),
		archivePath,
		WithCacheRoot(customCache),
	)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if !hasCustomCachePrefix(resolved.MetadataDir, customCache) {
		t.Fatalf("expected metadata dir under %q, got %q", customCache, resolved.MetadataDir)
	}

	// Default cache root MUST NOT have been used.
	if _, err := os.Stat(filepath.Join(tempHome, ".declarest", "metadata-bundles")); !os.IsNotExist(err) {
		t.Fatalf("expected default cache root untouched, got stat err=%v", err)
	}
}

func hasCustomCachePrefix(metadataDir, cacheRoot string) bool {
	rel, err := filepath.Rel(cacheRoot, metadataDir)
	if err != nil {
		return false
	}
	return !startsWithParent(rel)
}

func startsWithParent(rel string) bool {
	if rel == ".." {
		return true
	}
	if len(rel) >= 3 && rel[0] == '.' && rel[1] == '.' && (rel[2] == '/' || rel[2] == filepath.Separator) {
		return true
	}
	return false
}
