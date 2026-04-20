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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/crmarques/declarest/faults"
)

func TestResolveBundleShorthandUsesManifestAndCache(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/gzip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	previousBase := shorthandReleaseBaseURL
	shorthandReleaseBaseURL = server.URL
	defer func() {
		shorthandReleaseBaseURL = previousBase
	}()

	resolved, err := ResolveBundle(context.Background(), "keycloak-bundle:0.1.0")
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if !resolved.Shorthand {
		t.Fatal("expected shorthand resolution")
	}
	if resolved.Manifest.Name != "keycloak-bundle" {
		t.Fatalf("expected bundle name keycloak-bundle, got %q", resolved.Manifest.Name)
	}
	if _, err := os.Stat(filepath.Join(resolved.MetadataDir, "admin", "realms", "_", "metadata.json")); err != nil {
		t.Fatalf("expected metadata tree under resolved metadata directory: %v", err)
	}

	// Ensure cache is reused when source is unavailable.
	server.Close()
	resolvedAgain, err := ResolveBundle(context.Background(), "keycloak-bundle:v0.1.0")
	if err != nil {
		t.Fatalf("ResolveBundle cache hit returned error: %v", err)
	}
	if resolvedAgain.MetadataDir != resolved.MetadataDir {
		t.Fatalf("expected cached metadata directory %q, got %q", resolved.MetadataDir, resolvedAgain.MetadataDir)
	}
}

func TestResolveBundleFailsWhenMetadataRootMissing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	_, err := ResolveBundle(context.Background(), archivePath)
	if err == nil {
		t.Fatal("expected metadata root validation error")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error category, got %v", err)
	}
	if !strings.Contains(err.Error(), "metadata root") {
		t.Fatalf("expected metadata root error, got %v", err)
	}
}

func TestResolveBundleFailsWhenShorthandVersionDiffersFromManifest(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.2.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/gzip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	previousBase := shorthandReleaseBaseURL
	shorthandReleaseBaseURL = server.URL
	defer func() {
		shorthandReleaseBaseURL = previousBase
	}()

	_, err := ResolveBundle(context.Background(), "keycloak-bundle:0.1.0")
	if err == nil {
		t.Fatal("expected shorthand version mismatch error")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Fatalf("expected version mismatch error, got %v", err)
	}
}

func TestResolveBundleFailsWhenArtifactTemplateDiffersFromNamingContract(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/gzip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	previousBase := shorthandReleaseBaseURL
	shorthandReleaseBaseURL = server.URL
	defer func() {
		shorthandReleaseBaseURL = previousBase
	}()

	_, err := ResolveBundle(context.Background(), "keycloak-bundle:0.1.0")
	if err == nil {
		t.Fatal("expected artifact template validation error")
	}
	if !strings.Contains(err.Error(), "artifactTemplate") {
		t.Fatalf("expected artifact template error, got %v", err)
	}
}

func TestResolveBundleDeprecatedShorthandReturnsWarning(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
deprecated: true
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/gzip")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	previousBase := shorthandReleaseBaseURL
	shorthandReleaseBaseURL = server.URL
	defer func() {
		shorthandReleaseBaseURL = previousBase
	}()

	resolved, err := ResolveBundle(context.Background(), "keycloak-bundle:0.1.0")
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if strings.TrimSpace(resolved.DeprecatedWarning) == "" {
		t.Fatal("expected deprecated shorthand warning")
	}
}

func TestResolveBundleUsesManifestOpenAPIURL(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  openapi: https://www.keycloak.org/docs-api/26.4.7/rest-api/openapi.yaml
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if resolved.OpenAPI != "https://www.keycloak.org/docs-api/26.4.7/rest-api/openapi.yaml" {
		t.Fatalf("expected manifest openapi URL to be used, got %q", resolved.OpenAPI)
	}
}

func TestResolveBundleUsesPeerOpenAPIYamlWhenManifestOpenAPIMissing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
`,
		"openapi.yaml": `
openapi: 3.0.0
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if strings.TrimSpace(resolved.OpenAPI) == "" {
		t.Fatal("expected bundled openapi.yaml to be resolved")
	}
	if filepath.Base(resolved.OpenAPI) != "openapi.yaml" {
		t.Fatalf("expected resolved OpenAPI file path, got %q", resolved.OpenAPI)
	}
	if _, statErr := os.Stat(resolved.OpenAPI); statErr != nil {
		t.Fatalf("expected resolved OpenAPI file to exist: %v", statErr)
	}
}

func TestResolveBundleUsesMetadataRootOpenAPIWhenRootOpenAPIMissing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
`,
		"metadata/openapi.yaml": `
openapi: 3.0.3
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if strings.TrimSpace(resolved.OpenAPI) == "" {
		t.Fatal("expected metadata-root bundled openapi.yaml to be resolved")
	}
	if filepath.Base(resolved.OpenAPI) != "openapi.yaml" {
		t.Fatalf("expected metadata-root OpenAPI file path, got %q", resolved.OpenAPI)
	}
	if !strings.Contains(resolved.OpenAPI, string(filepath.Separator)+"metadata"+string(filepath.Separator)) {
		t.Fatalf("expected metadata-root OpenAPI path, got %q", resolved.OpenAPI)
	}
}

func TestResolveBundleUsesRecursiveOpenAPIFallbackWhenCommonLocationsMissing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
`,
		"docs/reference/openapi.yaml": `
openapi: 3.1.0
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if strings.TrimSpace(resolved.OpenAPI) == "" {
		t.Fatal("expected recursive bundled openapi.yaml fallback to be resolved")
	}
	if filepath.Base(resolved.OpenAPI) != "openapi.yaml" {
		t.Fatalf("expected recursive OpenAPI file path, got %q", resolved.OpenAPI)
	}
	if !strings.Contains(resolved.OpenAPI, string(filepath.Separator)+"docs"+string(filepath.Separator)+"reference"+string(filepath.Separator)) {
		t.Fatalf("expected recursive fallback OpenAPI path, got %q", resolved.OpenAPI)
	}
}

func TestResolveBundlePrefersManifestOpenAPIOverPeerOpenAPIYaml(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  openapi: https://www.keycloak.org/docs-api/26.4.7/rest-api/openapi.yaml
`,
		"openapi.yaml": `
openapi: 3.0.0
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	resolved, err := ResolveBundle(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if resolved.OpenAPI != "https://www.keycloak.org/docs-api/26.4.7/rest-api/openapi.yaml" {
		t.Fatalf("expected manifest OpenAPI URL precedence, got %q", resolved.OpenAPI)
	}
}

func TestResolveBundleFailsWhenManifestOpenAPIReferenceIsInvalid(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  openapi: ftp://example.com/openapi.yaml
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	_, err := ResolveBundle(context.Background(), archivePath)
	if err == nil {
		t.Fatal("expected invalid manifest openapi reference error")
	}
	if !strings.Contains(err.Error(), "declarest.openapi") {
		t.Fatalf("expected declarest.openapi validation error, got %v", err)
	}
}

func buildTestBundleArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	buffer := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for path, content := range files {
		data := []byte(content)
		header := &tar.Header{
			Name: filepath.ToSlash(path),
			Mode: 0o644,
			Size: int64(len(data)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header for %q: %v", path, err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatalf("failed to write tar data for %q: %v", path, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buffer.Bytes()
}

func TestParseShorthandRefRejectsInvalidSemver(t *testing.T) {
	_, _, ok := parseShorthandRef("keycloak-bundle:latest")
	if ok {
		t.Fatal("expected shorthand parsing to reject non-semver versions")
	}
}

func TestExpectedArtifactTemplate(t *testing.T) {
	if got, want := expectedArtifactTemplate("keycloak-bundle"), "keycloak-bundle-{version}.tar.gz"; got != want {
		t.Fatalf("expected artifact template %q, got %q", want, got)
	}
}

func TestNormalizeSemverKeepsCanonicalVersion(t *testing.T) {
	if got, err := normalizeSemver("v1.2.3"); err != nil || got != "1.2.3" {
		t.Fatalf("expected normalized semver 1.2.3, got %q err=%v", got, err)
	}
	if got, err := normalizeSemver("2.0.0-rc.1"); err != nil || got != "2.0.0-rc.1" {
		t.Fatalf("expected prerelease semver, got %q err=%v", got, err)
	}
}

func TestResolveBundleRejectsUnsupportedArchiveEntryTypes(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildBundleArchiveWithUnsupportedEntry(t)
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	_, err := ResolveBundle(context.Background(), archivePath)
	if err == nil {
		t.Fatal("expected unsupported entry type error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported entry type error, got %v", err)
	}
}

func buildBundleArchiveWithUnsupportedEntry(t *testing.T) []byte {
	t.Helper()

	buffer := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	entries := []struct {
		header *tar.Header
		data   []byte
	}{
		{
			header: &tar.Header{Name: "bundle.yaml", Mode: 0o644, Size: int64(len(`
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`))},
			data: []byte(`
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`),
		},
		{
			header: &tar.Header{Name: "metadata/admin", Typeflag: tar.TypeSymlink, Linkname: "/tmp/outside", Mode: 0o755},
		},
	}

	for _, entry := range entries {
		if err := tarWriter.WriteHeader(entry.header); err != nil {
			t.Fatalf("failed to write tar header %q: %v", entry.header.Name, err)
		}
		if len(entry.data) > 0 {
			if _, err := tarWriter.Write(entry.data); err != nil {
				t.Fatalf("failed to write tar data %q: %v", entry.header.Name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

func TestResolveBundleShorthandSourceURLContract(t *testing.T) {
	source, err := parseBundleSource("keycloak-bundle:v1.2.3")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if source.kind != sourceKindShort {
		t.Fatalf("expected shorthand source kind, got %q", source.kind)
	}
	expectedURL := "https://github.com/crmarques/declarest-bundle-keycloak/releases/download/v1.2.3/keycloak-bundle-1.2.3.tar.gz"
	if source.remoteURL != expectedURL {
		t.Fatalf("expected shorthand URL %q, got %q", expectedURL, source.remoteURL)
	}
	if source.cacheDirName != "keycloak-bundle-1.2.3" {
		t.Fatalf("expected shorthand cache dir keycloak-bundle-1.2.3, got %q", source.cacheDirName)
	}
}

func TestResolveBundleURLSourceUsesDeterministicCacheKey(t *testing.T) {
	sourceA, err := parseBundleSource("https://example.com/bundles/keycloak.tar.gz")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	sourceB, err := parseBundleSource("https://example.com/bundles/keycloak.tar.gz")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if sourceA.cacheDirName != sourceB.cacheDirName {
		t.Fatalf("expected deterministic cache dir names, got %q and %q", sourceA.cacheDirName, sourceB.cacheDirName)
	}
}

func TestResolveBundleURLSourceUsesVersionCacheKeyForVersionedArtifacts(t *testing.T) {
	sourceA, err := parseBundleSource("https://example.com/bundles/keycloak-bundle-1.2.3.tar.gz?X-Amz-Signature=a")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	sourceB, err := parseBundleSource("https://example.com/bundles/keycloak-bundle-1.2.3.tar.gz?X-Amz-Signature=b")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if got, want := sourceA.cacheDirName, "keycloak-bundle-1.2.3"; got != want {
		t.Fatalf("expected version cache dir %q, got %q", want, got)
	}
	if sourceA.cacheDirName != sourceB.cacheDirName {
		t.Fatalf("expected version cache dir reuse, got %q and %q", sourceA.cacheDirName, sourceB.cacheDirName)
	}
}

func TestResolveBundleLocalSourceUsesDeterministicCacheKey(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "my-bundle.tar.gz")
	if err := os.WriteFile(archivePath, []byte("not-used"), 0o600); err != nil {
		t.Fatalf("failed to write placeholder archive: %v", err)
	}

	sourceA, err := parseBundleSource(archivePath)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	sourceB, err := parseBundleSource(archivePath)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if sourceA.cacheDirName != sourceB.cacheDirName {
		t.Fatalf("expected deterministic cache dir names, got %q and %q", sourceA.cacheDirName, sourceB.cacheDirName)
	}
}

func TestResolveBundleLocalSourceUsesVersionCacheKeyForVersionedArtifacts(t *testing.T) {
	baseName := "keycloak-bundle-1.2.3.tar.gz"
	dirA := t.TempDir()
	dirB := t.TempDir()
	archivePathA := filepath.Join(dirA, baseName)
	archivePathB := filepath.Join(dirB, baseName)
	if err := os.WriteFile(archivePathA, []byte("not-used"), 0o600); err != nil {
		t.Fatalf("failed to write placeholder archive A: %v", err)
	}
	if err := os.WriteFile(archivePathB, []byte("not-used"), 0o600); err != nil {
		t.Fatalf("failed to write placeholder archive B: %v", err)
	}

	sourceA, err := parseBundleSource(archivePathA)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	sourceB, err := parseBundleSource(archivePathB)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if got, want := sourceA.cacheDirName, "keycloak-bundle-1.2.3"; got != want {
		t.Fatalf("expected version cache dir %q, got %q", want, got)
	}
	if sourceA.cacheDirName != sourceB.cacheDirName {
		t.Fatalf("expected version cache dir reuse, got %q and %q", sourceA.cacheDirName, sourceB.cacheDirName)
	}
}

func TestResolveBundleConcurrentVersionedLocalSourceInstallIsStable(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 1.2.3
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
`,
		"openapi.yaml": `
openapi: 3.0.0
paths: {}
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	baseName := "keycloak-bundle-1.2.3.tar.gz"
	archivePathA := filepath.Join(t.TempDir(), baseName)
	archivePathB := filepath.Join(t.TempDir(), baseName)
	if err := os.WriteFile(archivePathA, archive, 0o600); err != nil {
		t.Fatalf("failed to write archive A: %v", err)
	}
	if err := os.WriteFile(archivePathB, archive, 0o600); err != nil {
		t.Fatalf("failed to write archive B: %v", err)
	}

	source, err := parseBundleSource(archivePathA)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	cacheRoot, err := defaultCacheRoot()
	if err != nil {
		t.Fatalf("defaultCacheRoot returned error: %v", err)
	}
	cacheDir := filepath.Join(cacheRoot, source.cacheDirName)

	const (
		rounds  = 20
		workers = 8
	)

	for round := 0; round < rounds; round++ {
		if err := os.RemoveAll(cacheDir); err != nil {
			t.Fatalf("failed to reset cache dir: %v", err)
		}

		start := make(chan struct{})
		errs := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				<-start

				ref := archivePathA
				if worker%2 == 1 {
					ref = archivePathB
				}

				resolved, resolveErr := ResolveBundle(context.Background(), ref)
				if resolveErr != nil {
					errs <- fmt.Errorf("worker %d resolve error: %w", worker, resolveErr)
					return
				}
				if strings.TrimSpace(resolved.OpenAPI) == "" {
					errs <- fmt.Errorf("worker %d expected resolved OpenAPI source", worker)
					return
				}
				if _, statErr := os.Stat(resolved.OpenAPI); statErr != nil {
					errs <- fmt.Errorf("worker %d expected OpenAPI file: %w", worker, statErr)
					return
				}
				if _, statErr := os.Stat(filepath.Join(resolved.MetadataDir, "admin", "realms", "_", "metadata.json")); statErr != nil {
					errs <- fmt.Errorf("worker %d expected metadata tree: %w", worker, statErr)
					return
				}
			}(worker)
		}

		close(start)
		wg.Wait()
		close(errs)
		for resolveErr := range errs {
			t.Fatalf("round %d: %v", round, resolveErr)
		}
	}
}

func TestExtractTarGzRejectsExcessiveEntryCount(t *testing.T) {
	t.Parallel()

	buffer := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for i := 0; i <= maxArchiveEntries; i++ {
		name := fmt.Sprintf("file-%d.txt", i)
		data := []byte("x")
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(data)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatalf("failed to write tar data: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	dest := t.TempDir()
	err := extractTarGz(bytes.NewReader(buffer.Bytes()), dest)
	if err == nil {
		t.Fatal("expected error for excessive entry count")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(err.Error(), "too many entries") {
		t.Fatalf("expected 'too many entries' error, got %v", err)
	}
}

func TestExtractTarGzRejectsExcessiveTotalSize(t *testing.T) {
	t.Parallel()

	buffer := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	// Create files that individually fit under per-file limit but exceed total.
	fileSize := int64(maxArchiveFileSizeByte) // 64 MiB each
	numFiles := (maxTotalArchiveBytes / fileSize) + 1

	for i := int64(0); i < numFiles; i++ {
		name := fmt.Sprintf("large-%d.bin", i)
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: fileSize,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		// Write zero-filled data in chunks to avoid large allocations.
		chunk := make([]byte, 32*1024)
		remaining := fileSize
		for remaining > 0 {
			n := int64(len(chunk))
			if n > remaining {
				n = remaining
			}
			if _, err := tarWriter.Write(chunk[:n]); err != nil {
				t.Fatalf("failed to write tar data: %v", err)
			}
			remaining -= n
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	dest := t.TempDir()
	err := extractTarGz(bytes.NewReader(buffer.Bytes()), dest)
	if err == nil {
		t.Fatal("expected error for excessive total size")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(err.Error(), "maximum total size") {
		t.Fatalf("expected 'maximum total size' error, got %v", err)
	}
}

func TestResolveBundleEnforcesCompatibleDeclarestGate(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  compatibleDeclarest: ">=1.0.0"
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	_, err := ResolveBundle(context.Background(), archivePath, WithDeclarestVersion("0.5.0"))
	if err == nil {
		t.Fatal("expected compatibleDeclarest gate to reject older binary")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(err.Error(), "compatibleDeclarest") &&
		!strings.Contains(err.Error(), "requires declarest") {
		t.Fatalf("expected compat error to mention compatibleDeclarest, got %v", err)
	}
}

func TestResolveBundleAllowsCompatibleDeclarestGateWhenSatisfied(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  compatibleDeclarest: ">=1.0.0"
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	if _, err := ResolveBundle(context.Background(), archivePath, WithDeclarestVersion("v1.4.2")); err != nil {
		t.Fatalf("expected compatibleDeclarest gate to allow satisfying binary, got %v", err)
	}
}

func TestResolveBundleBypassesCompatibleDeclarestGateForDevBuild(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
  compatibleDeclarest: ">=99.0.0"
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatalf("failed to write test bundle archive: %v", err)
	}

	if _, err := ResolveBundle(context.Background(), archivePath, WithDeclarestVersion("dev")); err != nil {
		t.Fatalf("expected compatibleDeclarest gate to bypass dev build, got %v", err)
	}
}

func Example_expectedArtifactTemplate() {
	fmt.Println(expectedArtifactTemplate("keycloak-bundle"))
	// Output: keycloak-bundle-{version}.tar.gz
}
