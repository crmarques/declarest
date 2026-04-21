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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/crmarques/declarest/faults"
)

func TestParseOCIRefAcceptsCanonicalTag(t *testing.T) {
	source, err := parseBundleSource("oci://ghcr.io/crmarques/declarest-metadata-bundles/keycloak:0.0.1")
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if source.kind != sourceKindOCI {
		t.Fatalf("expected oci kind, got %q", source.kind)
	}
	if source.ociRegistry != "ghcr.io" {
		t.Fatalf("expected registry ghcr.io, got %q", source.ociRegistry)
	}
	if source.ociRepository != "crmarques/declarest-metadata-bundles/keycloak" {
		t.Fatalf("unexpected repository: %q", source.ociRepository)
	}
	if source.ociReference != "0.0.1" {
		t.Fatalf("expected reference 0.0.1, got %q", source.ociReference)
	}
	if source.ociByDigest {
		t.Fatal("expected tag reference, got digest flag")
	}
	if !strings.HasPrefix(source.cacheDirName, "keycloak-0.0.1") {
		t.Fatalf("expected cache dir prefix keycloak-0.0.1, got %q", source.cacheDirName)
	}
}

func TestParseOCIRefAcceptsDigestReference(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	source, err := parseBundleSource("oci://ghcr.io/org/bundle/keycloak@" + digest)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}
	if !source.ociByDigest {
		t.Fatal("expected digest reference")
	}
	if source.ociReference != digest {
		t.Fatalf("expected digest reference, got %q", source.ociReference)
	}
}

func TestParseOCIRefRejectsMissingTagOrDigest(t *testing.T) {
	_, err := parseBundleSource("oci://ghcr.io/crmarques/bundle/keycloak")
	if err == nil {
		t.Fatal("expected validation error for missing tag")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestParseOCIRefRejectsEmptyRegistry(t *testing.T) {
	_, err := parseBundleSource("oci:///path:tag")
	if err == nil {
		t.Fatal("expected validation error for empty registry")
	}
}

func TestResolveBundleOCIPullsArchive(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak
version: 0.0.1
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := newFakeOCIRegistry(t, ociArtifact{
		repository: "crmarques/declarest-metadata-bundles/keycloak",
		tag:        "0.0.1",
		layers: []ociFakeLayer{
			{
				mediaType: ociBundleLayerMediaType,
				content:   archive,
			},
		},
	})
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse fake registry url: %v", err)
	}

	source, err := parseBundleSource(
		fmt.Sprintf("oci://%s/crmarques/declarest-metadata-bundles/keycloak:0.0.1", serverURL.Host),
	)
	if err != nil {
		t.Fatalf("parseBundleSource returned error: %v", err)
	}

	repository := buildTestOCIRepository(t, source)
	manifest, err := fetchOCIBundleManifest(context.Background(), repository, source.ociReference)
	if err != nil {
		t.Fatalf("fetchOCIBundleManifest returned error: %v", err)
	}
	layer, err := selectOCIBundleLayer(manifest.Layers)
	if err != nil {
		t.Fatalf("selectOCIBundleLayer returned error: %v", err)
	}
	blob, err := repository.Fetch(context.Background(), layer)
	if err != nil {
		t.Fatalf("repository.Fetch returned error: %v", err)
	}
	defer func() {
		_ = blob.Close()
	}()
}

func TestResolveBundleOCIEndToEndInstallsAndCachesArchive(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	archive := buildTestBundleArchive(t, map[string]string{
		"bundle.yaml": `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak
version: 0.0.1
description: Keycloak metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  artifactTemplate: keycloak-{version}.tar.gz
`,
		"metadata/admin/realms/_/metadata.json": `{}`,
	})

	server := newFakeOCIRegistry(t, ociArtifact{
		repository: "crmarques/declarest-metadata-bundles/keycloak",
		tag:        "0.0.1",
		layers: []ociFakeLayer{
			{
				mediaType: ociBundleLayerMediaType,
				content:   archive,
			},
		},
	})
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse fake registry url: %v", err)
	}

	ref := fmt.Sprintf("oci://%s/crmarques/declarest-metadata-bundles/keycloak:0.0.1", serverURL.Host)

	ociPlainHTTPRegistries[serverURL.Host] = struct{}{}
	t.Cleanup(func() {
		delete(ociPlainHTTPRegistries, serverURL.Host)
	})

	resolved, err := ResolveBundle(context.Background(), ref)
	if err != nil {
		t.Fatalf("ResolveBundle returned error: %v", err)
	}
	if resolved.Manifest.Name != "keycloak" {
		t.Fatalf("expected bundle name keycloak, got %q", resolved.Manifest.Name)
	}
	if resolved.Manifest.Version != "0.0.1" {
		t.Fatalf("expected bundle version 0.0.1, got %q", resolved.Manifest.Version)
	}
	if _, err := os.Stat(filepath.Join(resolved.MetadataDir, "admin", "realms", "_", "metadata.json")); err != nil {
		t.Fatalf("expected metadata tree under resolved metadata directory: %v", err)
	}

	// Cache hit path: mutate the registry content to verify the cache is reused.
	server.Close()

	resolvedCached, err := ResolveBundle(context.Background(), ref)
	if err != nil {
		t.Fatalf("ResolveBundle cached hit returned error: %v", err)
	}
	if resolvedCached.MetadataDir != resolved.MetadataDir {
		t.Fatalf("expected cached metadata directory reuse, got %q vs %q", resolvedCached.MetadataDir, resolved.MetadataDir)
	}
}

func buildTestOCIRepository(t *testing.T, source bundleSource) *remote.Repository {
	t.Helper()

	reference := registry.Reference{
		Registry:   source.ociRegistry,
		Repository: source.ociRepository,
		Reference:  source.ociReference,
	}
	if err := reference.Validate(); err != nil {
		t.Fatalf("reference.Validate returned error: %v", err)
	}
	repository, err := remote.NewRepository(reference.String())
	if err != nil {
		t.Fatalf("remote.NewRepository returned error: %v", err)
	}
	repository.PlainHTTP = true
	repository.Client = &auth.Client{
		Client: &http.Client{Timeout: 5 * time.Second},
		Cache:  auth.NewCache(),
	}
	return repository
}

type ociFakeLayer struct {
	mediaType string
	content   []byte
}

type ociArtifact struct {
	repository string
	tag        string
	layers     []ociFakeLayer
}

type fakeOCIRegistry struct {
	*httptest.Server
	artifact ociArtifact
}

func (r *fakeOCIRegistry) Close() {
	if r.Server == nil {
		return
	}
	r.Server.Close()
	r.Server = nil
}

func newFakeOCIRegistry(t *testing.T, artifact ociArtifact) *fakeOCIRegistry {
	t.Helper()

	blobs := map[string]ociFakeLayer{}
	manifestLayers := make([]ocispec.Descriptor, 0, len(artifact.layers))
	for _, layer := range artifact.layers {
		layerDigestHex := sha256Digest(layer.content)
		blobs[layerDigestHex] = layer
		manifestLayers = append(manifestLayers, ocispec.Descriptor{
			MediaType: layer.mediaType,
			Digest:    digest.Digest("sha256:" + layerDigestHex),
			Size:      int64(len(layer.content)),
		})
	}
	configContent := []byte(`{"schemaVersion":1}`)
	configDigest := sha256Digest(configContent)
	blobs[configDigest] = ociFakeLayer{
		mediaType: "application/vnd.declarest.bundle.config.v1+json",
		content:   configContent,
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config: ocispec.Descriptor{
			MediaType: "application/vnd.declarest.bundle.config.v1+json",
			Digest:    digest.Digest("sha256:" + configDigest),
			Size:      int64(len(configContent)),
		},
		Layers: manifestLayers,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	manifestDigest := sha256Digest(manifestBytes)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v2/" {
			writer.WriteHeader(http.StatusOK)
			return
		}
		manifestPath := "/v2/" + artifact.repository + "/manifests/"
		blobPath := "/v2/" + artifact.repository + "/blobs/"
		switch {
		case strings.HasPrefix(request.URL.Path, manifestPath):
			reference := strings.TrimPrefix(request.URL.Path, manifestPath)
			if reference != artifact.tag && reference != "sha256:"+manifestDigest {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
			writer.Header().Set("Docker-Content-Digest", "sha256:"+manifestDigest)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(manifestBytes)
		case strings.HasPrefix(request.URL.Path, blobPath):
			reference := strings.TrimPrefix(request.URL.Path, blobPath)
			const prefix = "sha256:"
			if !strings.HasPrefix(reference, prefix) {
				http.NotFound(writer, request)
				return
			}
			entry, ok := blobs[strings.TrimPrefix(reference, prefix)]
			if !ok {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", entry.mediaType)
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(entry.content)
		default:
			http.NotFound(writer, request)
		}
	})

	server := httptest.NewServer(mux)
	return &fakeOCIRegistry{Server: server, artifact: artifact}
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
