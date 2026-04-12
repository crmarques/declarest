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

func TestSyncPolicyValidateNoOverlap(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	existing := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-a", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/customers"},
		},
	}
	candidate := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-b", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/customers/acme"},
		},
	}

	reconciler := &SyncPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
		Scheme: scheme,
	}

	if err := reconciler.validateNoOverlap(context.Background(), candidate); err == nil {
		t.Fatal("expected overlap error, got nil")
	}
}

func TestSyncPolicyValidateNoOverlapRejectsOverlapAcrossDifferentReferences(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	existing := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-a", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo-a"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server-a"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets-a"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/admin/realms/acme"},
		},
	}
	candidate := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-b", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo-b"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server-b"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets-b"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/admin/realms/acme/clients"},
		},
	}

	reconciler := &SyncPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
		Scheme: scheme,
	}

	if err := reconciler.validateNoOverlap(context.Background(), candidate); err == nil {
		t.Fatal("expected overlap error, got nil")
	}
}

func TestSyncPolicyValidateNoOverlapAllowsDistinctPathsWithSharedReferences(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	existing := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-a", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/admin/realms/a"},
		},
	}
	candidate := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-b", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/admin/realms/b"},
		},
	}

	reconciler := &SyncPolicyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
		Scheme: scheme,
	}

	if err := reconciler.validateNoOverlap(context.Background(), candidate); err != nil {
		t.Fatalf("expected no overlap error, got %v", err)
	}
}

func TestSyncPolicyMapperByResourceRepositoryUsesIndex(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	policyA := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-a", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo-a"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/"},
		},
	}
	policyB := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-b", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ResourceRepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "repo-b"},
			ManagedServiceRef:     declarestv1alpha1.NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        declarestv1alpha1.NamespacedObjectReference{Name: "secrets"},
			Source:                declarestv1alpha1.SyncPolicySource{Path: "/"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(policyA, policyB).
		WithIndex(&declarestv1alpha1.SyncPolicy{}, syncPolicyIndexResourceRepositoryRef, func(obj ctrlclient.Object) []string {
			item, ok := obj.(*declarestv1alpha1.SyncPolicy)
			if !ok {
				return nil
			}
			return []string{item.Spec.ResourceRepositoryRef.Name}
		}).
		Build()

	reconciler := &SyncPolicyReconciler{Client: fakeClient, Scheme: scheme}
	requests := reconciler.syncPoliciesForResourceRepository(
		context.Background(),
		&declarestv1alpha1.ResourceRepository{
			ObjectMeta: metav1.ObjectMeta{Name: "repo-a", Namespace: "default"},
		},
	)
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].NamespacedName != (types.NamespacedName{Namespace: "default", Name: "policy-a"}) {
		t.Fatalf("unexpected request: %#v", requests[0].NamespacedName)
	}
}
