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

package v1alpha1

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetadataBundleValidateURLVariants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"shorthand", "keycloak:0.0.1"},
		{"oci tag", "oci://ghcr.io/acme/declarest-metadata-bundles/keycloak:0.0.1"},
		{"oci digest", "oci://ghcr.io/acme/declarest-metadata-bundles/keycloak@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{"https tarball", "https://example.com/bundle.tar.gz"},
		{"http tarball", "http://example.com/bundle.tar.gz"},
		{"file url", "file:///srv/bundles/keycloak-0.0.1.tar.gz"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bundle := &MetadataBundle{
				Spec: MetadataBundleSpec{
					Source: MetadataBundleSource{URL: tc.url},
				},
			}
			if err := bundle.ValidateSpec(); err != nil {
				t.Fatalf("unexpected validation error for %q: %v", tc.url, err)
			}
		})
	}
}

func TestMetadataBundleValidateRejectsMultipleSources(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				URL: "https://example.com/bundle.tar.gz",
				PVC: &MetadataBundlePVCSource{
					Storage: StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "bundles"}},
					Path:    "bundle.tar.gz",
				},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for multiple sources")
	}
}

func TestMetadataBundleValidateRejectsMissingSource(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{Spec: MetadataBundleSpec{}}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error when source is missing")
	}
}

func TestMetadataBundleValidateRejectsEmptyURL(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{URL: "   "},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for empty url")
	}
}

func TestMetadataBundleValidateRejectsUnknownURLScheme(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{URL: "ftp://example.com/bundle.tar.gz"},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for unsupported url scheme")
	}
}

func TestMetadataBundleValidateRejectsParentTraversalPVCPath(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				PVC: &MetadataBundlePVCSource{
					Storage: StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "bundles"}},
					Path:    "../outside.tar.gz",
				},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for parent traversal in pvc path")
	}
}

func TestMetadataBundleValidateConfigMapSource(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				ConfigMap: &MetadataBundleConfigMapSource{
					Name: "keycloak-bundle",
					Key:  "bundle.tar.gz",
				},
			},
		},
	}
	if err := bundle.ValidateSpec(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestMetadataBundleValidateRejectsConfigMapMissingFields(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				ConfigMap: &MetadataBundleConfigMapSource{Name: "", Key: ""},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for empty configMap name/key")
	}
}

func TestMetadataBundleValidateOCISourceWithPullSecret(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				URL:           "oci://ghcr.io/acme/declarest-metadata-bundles/haproxy:0.0.1",
				PullSecretRef: &corev1.LocalObjectReference{Name: "ghcr-pull"},
			},
		},
	}
	if err := bundle.ValidateSpec(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestMetadataBundleValidateRejectsPullSecretOnNonOCI(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				URL:           "https://example.com/bundle.tar.gz",
				PullSecretRef: &corev1.LocalObjectReference{Name: "ghcr-pull"},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for pullSecretRef without oci:// url")
	}
}

func TestMetadataBundleValidateRejectsPullSecretMissingName(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				URL:           "oci://ghcr.io/acme/declarest-metadata-bundles/haproxy:0.0.1",
				PullSecretRef: &corev1.LocalObjectReference{Name: ""},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for empty pullSecretRef name")
	}
}

func TestMetadataBundleDefaultPollInterval(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{URL: "keycloak:0.0.1"},
		},
	}
	bundle.Default()
	if bundle.Spec.PollInterval == nil {
		t.Fatal("expected pollInterval to be defaulted")
	}
	if bundle.Spec.PollInterval.Duration != time.Hour {
		t.Fatalf("expected 1h default pollInterval, got %v", bundle.Spec.PollInterval.Duration)
	}
}

func TestMetadataBundleValidateRejectsNegativePollInterval(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source:       MetadataBundleSource{URL: "keycloak:0.0.1"},
			PollInterval: &metav1.Duration{Duration: -time.Minute},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for negative pollInterval")
	}
}
