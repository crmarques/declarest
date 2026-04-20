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
	"strings"
	"testing"

	"github.com/crmarques/declarest/faults"
)

const minimalBundleManifestYAML = `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.1
description: Test metadata bundle.
declarest:
  metadataRoot: metadata
`

func TestDecodeBundleManifestRejectsUnknownTopLevelKey(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + "home: https://example.com\n"

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected strict decode to reject unknown top-level key")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(err.Error(), "home") {
		t.Fatalf("expected error to name unknown key 'home', got %v", err)
	}
}

func TestDecodeBundleManifestRejectsUnknownDeclarestKey(t *testing.T) {
	manifestYAML := `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.1
description: Test metadata bundle.
declarest:
  shorthand: keycloak-bundle
  metadataRoot: metadata
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected strict decode to reject unknown declarest key")
	}
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(err.Error(), "shorthand") {
		t.Fatalf("expected error to name unknown key 'shorthand', got %v", err)
	}
}

func TestDecodeBundleManifestRejectsUnknownDistributionKey(t *testing.T) {
	manifestYAML := `
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.1
description: Test metadata bundle.
declarest:
  metadataRoot: metadata
distribution:
  repo: crmarques/declarest-bundle-keycloak
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected strict decode to reject unknown distribution key")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Fatalf("expected error to name unknown key 'repo', got %v", err)
	}
}

func TestDecodeBundleManifestAcceptsCompatibleFields(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleDeclarest: ">=0.1.0"
  compatibleManagedService:
    product: keycloak
    versions: ">=26.0.0 <27.0.0"
`

	manifest, err := DecodeBundleManifest([]byte(manifestYAML))
	if err != nil {
		t.Fatalf("expected manifest with compatible fields to decode, got %v", err)
	}
	if manifest.Declarest.CompatibleDeclarest != ">=0.1.0" {
		t.Fatalf("expected compatibleDeclarest preserved, got %q", manifest.Declarest.CompatibleDeclarest)
	}
	if manifest.Declarest.CompatibleManagedService.Product != "keycloak" {
		t.Fatalf("expected compatibleManagedService.product=keycloak, got %q", manifest.Declarest.CompatibleManagedService.Product)
	}
	if manifest.Declarest.CompatibleManagedService.Versions != ">=26.0.0 <27.0.0" {
		t.Fatalf("expected compatibleManagedService.versions preserved, got %q", manifest.Declarest.CompatibleManagedService.Versions)
	}
}

func TestDecodeBundleManifestRejectsInvalidCompatibleDeclarestConstraint(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleDeclarest: "not-a-constraint"
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected invalid compatibleDeclarest to fail decode")
	}
	if !strings.Contains(err.Error(), "compatibleDeclarest") {
		t.Fatalf("expected error to name compatibleDeclarest, got %v", err)
	}
}

func TestDecodeBundleManifestRejectsInvalidCompatibleManagedServiceVersions(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleManagedService:
    product: keycloak
    versions: "not-a-constraint"
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected invalid compatibleManagedService.versions to fail decode")
	}
	if !strings.Contains(err.Error(), "compatibleManagedService.versions") {
		t.Fatalf("expected error to name compatibleManagedService.versions, got %v", err)
	}
}

func TestDecodeBundleManifestRejectsCompatibleManagedServiceMissingVersions(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleManagedService:
    product: keycloak
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected compatibleManagedService.product without versions to fail decode")
	}
	if !strings.Contains(err.Error(), "compatibleManagedService.versions") {
		t.Fatalf("expected error to require compatibleManagedService.versions, got %v", err)
	}
}

func TestDecodeBundleManifestRejectsCompatibleManagedServiceMissingProduct(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleManagedService:
    versions: ">=26.0.0 <27.0.0"
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected compatibleManagedService.versions without product to fail decode")
	}
	if !strings.Contains(err.Error(), "compatibleManagedService.product") {
		t.Fatalf("expected error to require compatibleManagedService.product, got %v", err)
	}
}

func TestDecodeBundleManifestRejectsCompatibleManagedServiceProductPattern(t *testing.T) {
	manifestYAML := minimalBundleManifestYAML + `  compatibleManagedService:
    product: KeyCloak
    versions: ">=26.0.0 <27.0.0"
`

	_, err := DecodeBundleManifest([]byte(manifestYAML))
	if err == nil {
		t.Fatal("expected non-lowercase product to fail decode")
	}
	if !strings.Contains(err.Error(), "compatibleManagedService.product") {
		t.Fatalf("expected error to name compatibleManagedService.product, got %v", err)
	}
}
