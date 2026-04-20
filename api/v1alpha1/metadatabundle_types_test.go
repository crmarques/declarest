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

func TestMetadataBundleValidateShorthandSource(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{Shorthand: "keycloak:0.0.1"},
		},
	}
	if err := bundle.ValidateSpec(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestMetadataBundleValidateRejectsMultipleSources(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				Shorthand: "keycloak:0.0.1",
				URL:       "https://example.com/bundle.tar.gz",
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

func TestMetadataBundleValidateRejectsParentTraversalFilePath(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{
				File: &MetadataBundleFileSource{
					Storage: StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "bundles"}},
					Path:    "../outside.tar.gz",
				},
			},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for parent traversal in file path")
	}
}

func TestMetadataBundleDefaultPollInterval(t *testing.T) {
	t.Parallel()

	bundle := &MetadataBundle{
		Spec: MetadataBundleSpec{
			Source: MetadataBundleSource{Shorthand: "keycloak:0.0.1"},
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
			Source:       MetadataBundleSource{Shorthand: "keycloak:0.0.1"},
			PollInterval: &metav1.Duration{Duration: -time.Minute},
		},
	}
	if err := bundle.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for negative pollInterval")
	}
}
