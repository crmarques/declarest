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
	"context"
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCRDGeneratorMapperByMetadataBundleUsesIndex(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	generator := &declarestv1alpha1.CRDGenerator{
		ObjectMeta: metav1.ObjectMeta{Name: "realms", Namespace: "default"},
		Spec: declarestv1alpha1.CRDGeneratorSpec{
			Versions: []declarestv1alpha1.CRDGeneratorVersion{{
				Name:              "v1alpha1",
				MetadataBundleRef: declarestv1alpha1.NamespacedObjectReference{Name: "bundle"},
			}},
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(generator).
		WithIndex(&declarestv1alpha1.CRDGenerator{}, crdGeneratorMetadataBundleIndex, func(obj ctrlclient.Object) []string {
			item, ok := obj.(*declarestv1alpha1.CRDGenerator)
			if !ok {
				return nil
			}
			var names []string
			for _, version := range item.Spec.Versions {
				names = append(names, version.MetadataBundleRef.Name)
			}
			return names
		}).
		Build()

	reconciler := &CRDGeneratorReconciler{Client: fakeClient, Scheme: scheme}
	requests := reconciler.crdGeneratorsForMetadataBundle(
		context.Background(),
		&declarestv1alpha1.MetadataBundle{ObjectMeta: metav1.ObjectMeta{Name: "bundle", Namespace: "default"}},
	)
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].NamespacedName != (types.NamespacedName{Namespace: "default", Name: "realms"}) {
		t.Fatalf("unexpected request: %#v", requests[0].NamespacedName)
	}
}
