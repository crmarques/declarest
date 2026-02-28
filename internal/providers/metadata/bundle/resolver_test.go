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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
  metadataRoot: metadata
  metadataFileName: metadata.json
distribution:
  artifactTemplate: declarest-bundle-keycloak-{version}.tar.gz
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

	resolved, err := ResolveBundle(context.Background(), "keycloak:0.1.0")
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if !resolved.Shorthand {
		t.Fatal("expected shorthand resolution")
	}
	if resolved.Manifest.Name != "keycloak" {
		t.Fatalf("expected bundle name keycloak, got %q", resolved.Manifest.Name)
	}
	if _, err := os.Stat(filepath.Join(resolved.MetadataDir, "admin", "realms", "_", "metadata.json")); err != nil {
		t.Fatalf("expected metadata tree under resolved metadata directory: %v", err)
	}

	// Ensure cache is reused when source is unavailable.
	server.Close()
	resolvedAgain, err := ResolveBundle(context.Background(), "keycloak:v0.1.0")
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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
name: keycloak
version: 0.2.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
  metadataRoot: metadata
distribution:
  artifactTemplate: declarest-bundle-keycloak-{version}.tar.gz
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

	_, err := ResolveBundle(context.Background(), "keycloak:0.1.0")
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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

	_, err := ResolveBundle(context.Background(), "keycloak:0.1.0")
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
deprecated: true
declarest:
  shorthand: keycloak
  metadataRoot: metadata
distribution:
  artifactTemplate: declarest-bundle-keycloak-{version}.tar.gz
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

	resolved, err := ResolveBundle(context.Background(), "keycloak:0.1.0")
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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

func TestResolveBundlePrefersManifestOpenAPIOverPeerOpenAPIYaml(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archivePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
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
	_, _, ok := parseShorthandRef("keycloak:latest")
	if ok {
		t.Fatal("expected shorthand parsing to reject non-semver versions")
	}
}

func TestExpectedArtifactTemplate(t *testing.T) {
	if got, want := expectedArtifactTemplate("keycloak"), "declarest-bundle-keycloak-{version}.tar.gz"; got != want {
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
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
  metadataRoot: metadata
distribution:
  artifactTemplate: declarest-bundle-keycloak-{version}.tar.gz
`))},
			data: []byte(`
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak
version: 0.1.0
description: Keycloak metadata bundle.
declarest:
  shorthand: keycloak
  metadataRoot: metadata
distribution:
  artifactTemplate: declarest-bundle-keycloak-{version}.tar.gz
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
	source, err := parseBundleSource("keycloak:v1.2.3")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if source.kind != sourceKindShort {
		t.Fatalf("expected shorthand source kind, got %q", source.kind)
	}
	expectedURL := "https://github.com/crmarques/declarest-bundle-keycloak/releases/download/v1.2.3/declarest-bundle-keycloak-1.2.3.tar.gz"
	if source.remoteURL != expectedURL {
		t.Fatalf("expected shorthand URL %q, got %q", expectedURL, source.remoteURL)
	}
	if source.cacheDirName != "declarest-bundle-keycloak-1.2.3" {
		t.Fatalf("expected shorthand cache dir declarest-bundle-keycloak-1.2.3, got %q", source.cacheDirName)
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

func Example_expectedArtifactTemplate() {
	fmt.Println(expectedArtifactTemplate("keycloak"))
	// Output: declarest-bundle-keycloak-{version}.tar.gz
}
