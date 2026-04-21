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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/httpclient"
)

const ociBundleLayerMediaType = "application/vnd.declarest.bundle.v1.tar+gzip"

// ociPlainHTTPRegistries is a test-only hook that lets unit tests point the
// OCI resolver at an httptest.Server. It MUST remain empty in production.
var ociPlainHTTPRegistries = map[string]struct{}{}

var ociBundleLayerMediaTypes = map[string]struct{}{
	ociBundleLayerMediaType:                             {},
	"application/vnd.oci.image.layer.v1.tar+gzip":       {},
	"application/vnd.docker.image.rootfs.diff.tar.gzip": {},
	"application/gzip":                                  {},
	"application/x-gzip":                                {},
	"application/x-tar+gzip":                            {},
}

func openOCIBundleStream(ctx context.Context, source bundleSource, opts bundleResolverOptions) (io.ReadCloser, error) {
	httpClient, err := httpclient.Build(httpclient.Options{
		Timeout:      120 * time.Second,
		Proxy:        opts.proxy,
		ProxyScope:   "metadata.proxy",
		ProxyRuntime: opts.runtime,
	})
	if err != nil {
		return nil, err
	}

	repository, err := buildOCIRepository(source, httpClient, opts)
	if err != nil {
		return nil, err
	}

	manifest, err := fetchOCIBundleManifest(ctx, repository, source.ociReference)
	if err != nil {
		return nil, err
	}

	layerDescriptor, err := selectOCIBundleLayer(manifest.Layers)
	if err != nil {
		return nil, err
	}

	blob, err := repository.Fetch(ctx, layerDescriptor)
	if err != nil {
		return nil, classifyOCIError("failed to fetch metadata bundle OCI blob", err)
	}
	return blob, nil
}

func buildOCIRepository(source bundleSource, httpClient *http.Client, opts bundleResolverOptions) (*remote.Repository, error) {
	reference := registry.Reference{
		Registry:   source.ociRegistry,
		Repository: source.ociRepository,
		Reference:  source.ociReference,
	}
	if err := reference.Validate(); err != nil {
		return nil, faults.Invalid("metadata bundle OCI reference is invalid", err)
	}

	repository, err := remote.NewRepository(reference.String())
	if err != nil {
		return nil, faults.Invalid("metadata bundle OCI reference is invalid", err)
	}
	authClient := &auth.Client{
		Client: httpClient,
		Header: http.Header{
			"User-Agent": []string{"declarest/bundle-resolver"},
		},
		Cache: auth.NewCache(),
	}
	if len(opts.registryCredentials) > 0 {
		authClient.Credential = buildStaticCredentialFunc(opts.registryCredentials)
	}
	repository.Client = authClient
	if _, ok := ociPlainHTTPRegistries[source.ociRegistry]; ok {
		repository.PlainHTTP = true
	}
	return repository, nil
}

// buildStaticCredentialFunc returns a CredentialFunc that answers from the
// supplied RegistryCredential list. Hosts are compared case-insensitively and
// only full `host[:port]` matches resolve; unknown hosts fall back to
// EmptyCredential, letting ORAS attempt anonymous access.
func buildStaticCredentialFunc(credentials []RegistryCredential) auth.CredentialFunc {
	lookup := make(map[string]auth.Credential, len(credentials))
	for _, credential := range credentials {
		host := strings.ToLower(strings.TrimSpace(credential.Registry))
		if host == "" {
			continue
		}
		lookup[host] = auth.Credential{
			Username: credential.Username,
			Password: credential.Password,
		}
	}
	return func(_ context.Context, hostport string) (auth.Credential, error) {
		if credential, ok := lookup[strings.ToLower(strings.TrimSpace(hostport))]; ok {
			return credential, nil
		}
		return auth.EmptyCredential, nil
	}
}

func fetchOCIBundleManifest(ctx context.Context, repository *remote.Repository, reference string) (ocispec.Manifest, error) {
	descriptor, manifestBody, err := repository.FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Manifest{}, classifyOCIError("failed to fetch metadata bundle OCI manifest", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, manifestBody)
		_ = manifestBody.Close()
	}()

	switch descriptor.MediaType {
	case ocispec.MediaTypeImageManifest,
		"application/vnd.docker.distribution.manifest.v2+json":
	default:
		return ocispec.Manifest{}, faults.Invalid(
			fmt.Sprintf("metadata bundle OCI reference resolved to unsupported media type %q", descriptor.MediaType),
			nil,
		)
	}

	body, err := io.ReadAll(io.LimitReader(manifestBody, 1<<20))
	if err != nil {
		return ocispec.Manifest{}, faults.Transport("failed to read metadata bundle OCI manifest", err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return ocispec.Manifest{}, faults.Invalid("metadata bundle OCI manifest is not valid JSON", err)
	}
	return manifest, nil
}

func selectOCIBundleLayer(layers []ocispec.Descriptor) (ocispec.Descriptor, error) {
	for _, layer := range layers {
		if _, ok := ociBundleLayerMediaTypes[layer.MediaType]; ok {
			if strings.TrimSpace(layer.Digest.String()) == "" {
				return ocispec.Descriptor{}, faults.Invalid("metadata bundle OCI layer digest is empty", nil)
			}
			return layer, nil
		}
	}
	return ocispec.Descriptor{}, faults.Invalid("metadata bundle OCI manifest has no tar+gzip layer", nil)
}

func classifyOCIError(message string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errdef.ErrNotFound) {
		return faults.NotFound(message, err)
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") {
		return faults.Auth(message, err)
	}
	return faults.Transport(message, err)
}
