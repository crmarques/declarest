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
)

func validCRDGenerator() *CRDGenerator {
	return &CRDGenerator{
		Spec: CRDGeneratorSpec{
			Group: "keycloak.example.com",
			Scope: CRDGeneratorScopeNamespaced,
			Names: CRDGeneratorNames{
				Kind:     "Realm",
				Plural:   "realms",
				Singular: "realm",
			},
			Versions: []CRDGeneratorVersion{
				{
					Name:              "v1alpha1",
					Served:            true,
					Storage:           true,
					MetadataBundleRef: NamespacedObjectReference{Name: "keycloak-0.0.1"},
					CollectionPath:    "/admin/realms",
				},
			},
		},
	}
}

func TestCRDGeneratorValidateHappyPath(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	if err := generator.ValidateSpec(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCRDGeneratorRejectsReservedGroup(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Group = "keycloak.declarest.io"
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for reserved group suffix")
	}
}

func TestCRDGeneratorRejectsReservedKindWithinDeclarest(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Group = "declarest.io"
	generator.Spec.Names.Kind = "ManagedService"
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for reserved kind")
	}
}

func TestCRDGeneratorRequiresSingleStorageVersion(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Versions = append(generator.Spec.Versions, CRDGeneratorVersion{
		Name:              "v1alpha2",
		Served:            true,
		Storage:           true,
		MetadataBundleRef: NamespacedObjectReference{Name: "keycloak-0.0.2"},
		CollectionPath:    "/admin/realms",
	})
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for multiple storage versions")
	}
}

func TestCRDGeneratorRequiresAtLeastOneServedVersion(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Versions[0].Served = false
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error when no served version is declared")
	}
}

func TestCRDGeneratorRejectsInvalidVersionName(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Versions[0].Name = "V1"
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error for invalid version name")
	}
}

func TestCRDGeneratorRejectsMissingBundleRef(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Spec.Versions[0].MetadataBundleRef = NamespacedObjectReference{}
	if err := generator.ValidateSpec(); err == nil {
		t.Fatal("expected validation error when metadataBundleRef is empty")
	}
}

func TestCRDGeneratorDefaultFillsListKind(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	generator.Default()
	if generator.Spec.Names.ListKind != "RealmList" {
		t.Fatalf("expected listKind RealmList, got %q", generator.Spec.Names.ListKind)
	}
	if generator.Spec.Conversion == nil || generator.Spec.Conversion.Strategy != CRDGeneratorConversionNone {
		t.Fatalf("expected conversion strategy to default to None, got %#v", generator.Spec.Conversion)
	}
}

func TestGeneratedCRDName(t *testing.T) {
	t.Parallel()

	generator := validCRDGenerator()
	if got := generator.GeneratedCRDName(); got != "realms.keycloak.example.com" {
		t.Fatalf("unexpected CRD name: %q", got)
	}
}
