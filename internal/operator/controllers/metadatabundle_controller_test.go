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
	"encoding/base64"
	"strings"
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetadataBundleSourceRefURLPassthrough(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
		want string
	}{
		{"oci tag", "oci://ghcr.io/acme/bundle:0.0.1", "oci://ghcr.io/acme/bundle:0.0.1"},
		{"https", "https://example.com/bundle.tar.gz", "https://example.com/bundle.tar.gz"},
		{"shorthand", "bundle:0.0.1", "bundle:0.0.1"},
		{"file", "file:///srv/bundles/bundle.tar.gz", "file:///srv/bundles/bundle.tar.gz"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bundle := &declarestv1alpha1.MetadataBundle{
				Spec: declarestv1alpha1.MetadataBundleSpec{
					Source: declarestv1alpha1.MetadataBundleSource{URL: tc.url},
				},
			}
			got, err := metadataBundleSourceRef(bundle)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMetadataBundleSourceRefPVCJoin(t *testing.T) {
	t.Parallel()

	bundle := &declarestv1alpha1.MetadataBundle{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "haproxy-bundle"},
		Spec: declarestv1alpha1.MetadataBundleSpec{
			Source: declarestv1alpha1.MetadataBundleSource{
				PVC: &declarestv1alpha1.MetadataBundlePVCSource{
					Storage: declarestv1alpha1.StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "bundle-store"}},
					Path:    "haproxy/0.0.1/bundle.tar.gz",
				},
			},
		},
	}
	got, err := metadataBundleSourceRef(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/var/lib/declarest/bundles/ops/bundle-store/haproxy/0.0.1/bundle.tar.gz"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMetadataBundleSourceRefConfigMapURL(t *testing.T) {
	t.Parallel()

	bundle := &declarestv1alpha1.MetadataBundle{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "haproxy-bundle"},
		Spec: declarestv1alpha1.MetadataBundleSpec{
			Source: declarestv1alpha1.MetadataBundleSource{
				ConfigMap: &declarestv1alpha1.MetadataBundleConfigMapSource{
					Name: "haproxy-bundle",
					Key:  "bundle.tar.gz",
				},
			},
		},
	}
	got, err := metadataBundleSourceRef(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "configmap://ops/haproxy-bundle/bundle.tar.gz"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestParseDockerConfigAuthsAuthField(t *testing.T) {
	t.Parallel()

	creds := base64.StdEncoding.EncodeToString([]byte("alice:pat123"))
	payload := []byte(`{"auths":{"ghcr.io":{"auth":"` + creds + `"}}}`)
	auths, err := parseDockerConfigAuths(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth entry, got %d", len(auths))
	}
	if auths[0].Registry != "ghcr.io" {
		t.Fatalf("unexpected registry: %q", auths[0].Registry)
	}
	if auths[0].Username != "alice" || auths[0].Password != "pat123" {
		t.Fatalf("unexpected creds: %+v", auths[0])
	}
}

func TestParseDockerConfigAuthsExplicitUserPass(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"auths":{"https://index.docker.io/v1/":{"username":"bob","password":"s3cret"}}}`)
	auths, err := parseDockerConfigAuths(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth entry, got %d", len(auths))
	}
	if auths[0].Registry != "index.docker.io" {
		t.Fatalf("expected host normalization, got %q", auths[0].Registry)
	}
	if auths[0].Username != "bob" || auths[0].Password != "s3cret" {
		t.Fatalf("unexpected creds: %+v", auths[0])
	}
}

func TestParseDockerConfigAuthsRejectsMalformedAuth(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"auths":{"ghcr.io":{"auth":"notbase64!!"}}}`)
	if _, err := parseDockerConfigAuths(payload); err == nil {
		t.Fatal("expected error for malformed base64 auth")
	}
}

func TestExtractConfigMapBundleBytesBinaryData(t *testing.T) {
	t.Parallel()

	content := []byte{0x1f, 0x8b, 0x08} // gzip magic; content is opaque to the extractor
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "haproxy"},
		BinaryData: map[string][]byte{"bundle.tar.gz": content},
	}
	got, err := extractConfigMapBundleBytes(cm, "bundle.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("unexpected bytes: %x", got)
	}
}

func TestExtractConfigMapBundleBytesDataBase64(t *testing.T) {
	t.Parallel()

	content := []byte("hello bundle")
	encoded := base64.StdEncoding.EncodeToString(content)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "haproxy"},
		Data:       map[string]string{"bundle.tar.gz": encoded},
	}
	got, err := extractConfigMapBundleBytes(cm, "bundle.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("unexpected bytes: %q", got)
	}
}

func TestExtractConfigMapBundleBytesMissingKey(t *testing.T) {
	t.Parallel()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "haproxy"},
		BinaryData: map[string][]byte{"other": {0x00}},
	}
	_, err := extractConfigMapBundleBytes(cm, "bundle.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "bundle.tar.gz") {
		t.Fatalf("error should mention missing key, got %v", err)
	}
}

func TestNormalizeRegistryHost(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"GHCR.IO":                    "ghcr.io",
		"https://ghcr.io":            "ghcr.io",
		"https://index.docker.io/v1": "index.docker.io",
		"quay.io:443":                "quay.io:443",
	}
	for input, want := range cases {
		if got := normalizeRegistryHost(input); got != want {
			t.Fatalf("normalizeRegistryHost(%q)=%q, want %q", input, got, want)
		}
	}
}
