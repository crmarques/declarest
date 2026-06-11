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
	"github.com/crmarques/declarest/internal/bootstrap"
	"github.com/crmarques/declarest/managedservice"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
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

func TestSyncPolicyMapperByMetadataBundleFansOutThroughManagedServices(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}

	managedService := &declarestv1alpha1.ManagedService{
		ObjectMeta: metav1.ObjectMeta{Name: "server", Namespace: "default"},
		Spec: declarestv1alpha1.ManagedServiceSpec{
			Metadata: declarestv1alpha1.DeclaRESTMetadataArtifact{
				BundleRef: &declarestv1alpha1.NamespacedObjectReference{Name: "bundle"},
			},
		},
	}
	policy := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ManagedServiceRef: declarestv1alpha1.NamespacedObjectReference{Name: "server"},
		},
	}
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(managedService, policy).
		WithIndex(&declarestv1alpha1.ManagedService{}, syncPolicyIndexManagedServiceMetadataBundleRef, func(obj ctrlclient.Object) []string {
			item, ok := obj.(*declarestv1alpha1.ManagedService)
			if !ok || item.Spec.Metadata.BundleRef == nil {
				return nil
			}
			return []string{item.Spec.Metadata.BundleRef.Name}
		}).
		WithIndex(&declarestv1alpha1.SyncPolicy{}, syncPolicyIndexManagedServiceRef, func(obj ctrlclient.Object) []string {
			item, ok := obj.(*declarestv1alpha1.SyncPolicy)
			if !ok {
				return nil
			}
			return []string{item.Spec.ManagedServiceRef.Name}
		}).
		Build()

	reconciler := &SyncPolicyReconciler{Client: fakeClient, Scheme: scheme}
	requests := reconciler.syncPoliciesForMetadataBundle(
		context.Background(),
		&declarestv1alpha1.MetadataBundle{ObjectMeta: metav1.ObjectMeta{Name: "bundle", Namespace: "default"}},
	)
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].NamespacedName != (types.NamespacedName{Namespace: "default", Name: "policy"}) {
		t.Fatalf("unexpected request: %#v", requests[0].NamespacedName)
	}
}

func TestSyncPolicyApplySkipsCRDGeneratorOwnedTargets(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	index.Register("default", "keycloak", "/admin/realms", "/admin/realms/acme", "", ConflictSource{
		CRDGeneratorNamespace: "default",
		CRDGeneratorName:      "realm-generator",
		GeneratedKind:         "Realm",
		LogicalPath:           "/admin/realms/acme",
	})
	policy := &declarestv1alpha1.SyncPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "git-sync", Namespace: "default"},
		Spec: declarestv1alpha1.SyncPolicySpec{
			ManagedServiceRef: declarestv1alpha1.NamespacedObjectReference{Name: "keycloak"},
		},
	}
	orchestrator := &conflictAwareOrchestrator{
		local: []resource.Resource{
			{LogicalPath: "/admin/realms/acme", CollectionPath: "/admin/realms"},
			{LogicalPath: "/admin/realms/other", CollectionPath: "/admin/realms"},
		},
	}
	recon := &syncPolicyReconciliation{
		SyncPolicyReconciler: &SyncPolicyReconciler{Conflicts: index},
		ctx:                  context.Background(),
		policy:               policy,
	}

	targeted, applied, err := recon.applyChanges(bootstrap.Session{Orchestrator: orchestrator}, &syncExecutionPlan{
		ApplyTargets: []syncApplyTarget{{Path: "/admin/realms", Recursive: true}},
	})
	if err != nil {
		t.Fatalf("applyChanges() unexpected error: %v", err)
	}
	if targeted != 2 || applied != 1 {
		t.Fatalf("expected targeted=2 applied=1, got targeted=%d applied=%d", targeted, applied)
	}
	if len(orchestrator.applied) != 1 || orchestrator.applied[0] != "/admin/realms/other" {
		t.Fatalf("expected only non-owned target to be applied, got %#v", orchestrator.applied)
	}
	if condition := findCondition(policy.Status.Conditions, declarestv1alpha1.ConditionTypeConflicting); condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected Conflicting=True condition, got %#v", policy.Status.Conditions)
	}
}

func TestSyncPolicyPruneSkipsCRDGeneratorOwnedRemoteResources(t *testing.T) {
	t.Parallel()

	index := NewConflictIndex()
	index.Register("default", "keycloak", "/admin/realms", "/admin/realms/acme", "remote-acme", ConflictSource{
		CRDGeneratorNamespace: "default",
		CRDGeneratorName:      "realm-generator",
		GeneratedKind:         "Realm",
		LogicalPath:           "/admin/realms/acme",
		RemoteID:              "remote-acme",
	})
	orchestrator := &conflictAwareOrchestrator{
		remote: []resource.Resource{
			{LogicalPath: "/admin/realms/acme", CollectionPath: "/admin/realms", RemoteID: "remote-acme"},
			{LogicalPath: "/admin/realms/other", CollectionPath: "/admin/realms", RemoteID: "remote-other"},
		},
	}
	recon := &syncPolicyReconciliation{
		SyncPolicyReconciler: &SyncPolicyReconciler{Conflicts: index},
		ctx:                  context.Background(),
		policy: &declarestv1alpha1.SyncPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "git-sync", Namespace: "default"},
			Spec: declarestv1alpha1.SyncPolicySpec{
				ManagedServiceRef: declarestv1alpha1.NamespacedObjectReference{Name: "keycloak"},
			},
		},
	}

	deleted, err := recon.pruneRemote(context.Background(), orchestrator, "/admin/realms", true)
	if err != nil {
		t.Fatalf("pruneRemote() unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one non-owned remote resource to be pruned, got %d", deleted)
	}
	if len(orchestrator.deleted) != 1 || orchestrator.deleted[0] != "/admin/realms/other" {
		t.Fatalf("expected only non-owned target to be pruned, got %#v", orchestrator.deleted)
	}
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for idx := range conditions {
		if conditions[idx].Type == conditionType {
			return &conditions[idx]
		}
	}
	return nil
}

type conflictAwareOrchestrator struct {
	local   []resource.Resource
	remote  []resource.Resource
	applied []string
	deleted []string
}

func (o *conflictAwareOrchestrator) GetLocal(_ context.Context, logicalPath string) (resource.Content, error) {
	for _, item := range o.local {
		if item.LogicalPath == logicalPath {
			return resource.Content{Value: item.Payload, Descriptor: item.PayloadDescriptor}, nil
		}
	}
	return resource.Content{}, nil
}

func (o *conflictAwareOrchestrator) ListLocal(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return append([]resource.Resource(nil), o.local...), nil
}

func (o *conflictAwareOrchestrator) GetRemote(_ context.Context, logicalPath string) (resource.Content, error) {
	for _, item := range o.remote {
		if item.LogicalPath == logicalPath {
			return resource.Content{Value: item.Payload, Descriptor: item.PayloadDescriptor}, nil
		}
	}
	return resource.Content{}, nil
}

func (o *conflictAwareOrchestrator) ListRemote(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return append([]resource.Resource(nil), o.remote...), nil
}

func (o *conflictAwareOrchestrator) GetOpenAPISpec(context.Context) (resource.Content, error) {
	return resource.Content{}, nil
}

func (o *conflictAwareOrchestrator) Request(context.Context, managedservice.RequestSpec) (resource.Content, error) {
	return resource.Content{}, nil
}

func (o *conflictAwareOrchestrator) Save(context.Context, string, resource.Content) error {
	return nil
}

func (o *conflictAwareOrchestrator) Apply(ctx context.Context, logicalPath string, policy orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	item := resource.Resource{LogicalPath: logicalPath, CollectionPath: collectionPathForSyncPath(logicalPath)}
	for _, candidate := range o.local {
		if candidate.LogicalPath == logicalPath {
			item = candidate
			break
		}
	}
	if policy.Conflict != nil {
		skip, _ := policy.Conflict(ctx, orchestratordomain.ConflictCheck{
			LogicalPath:    item.LogicalPath,
			CollectionPath: item.CollectionPath,
			RemoteID:       item.RemoteID,
		})
		if skip {
			return resource.Resource{}, nil
		}
	}
	o.applied = append(o.applied, logicalPath)
	return item, nil
}

func (o *conflictAwareOrchestrator) ApplyWithContent(context.Context, string, resource.Content, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (o *conflictAwareOrchestrator) Create(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (o *conflictAwareOrchestrator) Update(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (o *conflictAwareOrchestrator) Delete(_ context.Context, logicalPath string, _ orchestratordomain.DeletePolicy) error {
	o.deleted = append(o.deleted, logicalPath)
	return nil
}

func (o *conflictAwareOrchestrator) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}

func (o *conflictAwareOrchestrator) Template(context.Context, string, resource.Content) (resource.Content, error) {
	return resource.Content{}, nil
}
