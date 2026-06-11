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
	"github.com/crmarques/declarest/config"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/internal/bootstrap"
	"github.com/crmarques/declarest/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGeneratedResourceApplyUsesExplicitPayloadAndBundleContext(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add declarest scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}

	metadataRoot := t.TempDir()
	managedService := &declarestv1alpha1.ManagedService{
		ObjectMeta: metav1.ObjectMeta{Name: "keycloak", Namespace: "tenant-a"},
		Spec: declarestv1alpha1.ManagedServiceSpec{
			HTTP: declarestv1alpha1.ManagedServiceHTTP{
				BaseURL: "https://keycloak.example.test",
				Auth: declarestv1alpha1.ManagedServiceAuth{
					CustomHeaders: []declarestv1alpha1.ManagedServiceHeaderAuth{{
						Header: "Authorization",
						ValueRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "managed-service-auth"},
							Key:                  "token",
						},
					}},
				},
			},
		},
	}
	authSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-service-auth", Namespace: "tenant-a"},
		Data:       map[string][]byte{"token": []byte("Bearer abc")},
	}
	bundle := &declarestv1alpha1.MetadataBundle{
		ObjectMeta: metav1.ObjectMeta{Name: "keycloak-bundle", Namespace: "operators"},
		Spec: declarestv1alpha1.MetadataBundleSpec{
			Source: declarestv1alpha1.MetadataBundleSource{URL: "oci://example.test/keycloak@sha256:abc"},
		},
		Status: declarestv1alpha1.MetadataBundleStatus{
			CachePath: metadataRoot,
			Conditions: []metav1.Condition{{
				Type:   declarestv1alpha1.ConditionTypeReady,
				Status: metav1.ConditionTrue,
				Reason: conditionReasonReady,
			}},
		},
	}

	generator := &declarestv1alpha1.CRDGenerator{
		ObjectMeta: metav1.ObjectMeta{Name: "realms", Namespace: "operators"},
	}
	version := &declarestv1alpha1.CRDGeneratorVersion{
		Name:              "v1alpha1",
		CollectionPath:    "/admin/realms",
		MetadataBundleRef: declarestv1alpha1.NamespacedObjectReference{Name: "keycloak-bundle"},
	}
	generated := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "resources.example.test/v1alpha1",
			"kind":       "Realm",
			"metadata": map[string]any{
				"name":      "acme",
				"namespace": "tenant-a",
			},
			"spec": map[string]any{
				"managedServiceRef": map[string]any{"name": "keycloak"},
				"payload":           map[string]any{"enabled": true},
			},
		},
	}

	var resolved config.Context
	var captured mutateapp.Request
	reconciler := &GeneratedResourceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(managedService, authSecret, bundle).
			Build(),
		SessionBuilder: func(ctx config.Context) (bootstrap.Session, error) {
			resolved = ctx
			return bootstrap.Session{}, nil
		},
		MutationExecutor: func(_ context.Context, _ mutateapp.Dependencies, req mutateapp.Request) (mutateapp.Result, error) {
			captured = req
			return mutateapp.Result{
				ResolvedPath:  req.LogicalPath,
				TargetedCount: 1,
				Items: []resource.Resource{{
					LogicalPath:    req.LogicalPath,
					CollectionPath: "/admin/realms",
					RemoteID:       "remote-acme",
				}},
			}, nil
		},
	}

	applied, err := reconciler.applyGeneratedResource(
		context.Background(),
		generator,
		version,
		generated,
		"tenant-a",
		"keycloak",
		"/admin/realms",
		"/admin/realms/acme",
	)
	if err != nil {
		t.Fatalf("applyGeneratedResource() unexpected error: %v", err)
	}

	if applied.RemoteID != "remote-acme" {
		t.Fatalf("expected remote ID from mutation result, got %q", applied.RemoteID)
	}
	if resolved.Name != "acme" {
		t.Fatalf("expected runtime context name acme, got %q", resolved.Name)
	}
	if resolved.Repository.Filesystem != nil || resolved.Repository.Git != nil {
		t.Fatalf("generated resource runtime context must not configure a repository: %#v", resolved.Repository)
	}
	if resolved.ManagedService == nil || resolved.ManagedService.HTTP == nil || resolved.ManagedService.HTTP.BaseURL != "https://keycloak.example.test" {
		t.Fatalf("managed service config was not populated from managedServiceRef: %#v", resolved.ManagedService)
	}
	if resolved.Metadata.BaseDir != metadataRoot {
		t.Fatalf("expected metadata bundle cache path %q, got %q", metadataRoot, resolved.Metadata.BaseDir)
	}
	if captured.Operation != mutateapp.OperationApply || !captured.HasExplicitInput {
		t.Fatalf("expected explicit apply request, got operation=%q explicit=%v", captured.Operation, captured.HasExplicitInput)
	}
	if captured.LogicalPath != "/admin/realms/acme" {
		t.Fatalf("expected generated logical path, got %q", captured.LogicalPath)
	}
	payload, ok := captured.Value.Value.(map[string]any)
	if !ok || payload["enabled"] != true {
		t.Fatalf("expected spec.payload to be passed as mutation input, got %#v", captured.Value.Value)
	}
}

func TestGeneratedResourceMetadataAddsOwnerAndFinalizer(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "resources.example.test", Version: "v1alpha1", Kind: "Realm"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetNamespace("tenant-a")
	obj.SetName("acme")

	generator := &declarestv1alpha1.CRDGenerator{
		ObjectMeta: metav1.ObjectMeta{Name: "realms", Namespace: "operators"},
	}
	reconciler := &GeneratedResourceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(obj).Build(),
	}

	if err := reconciler.ensureGeneratedResourceMetadata(context.Background(), obj, generator); err != nil {
		t.Fatalf("ensureGeneratedResourceMetadata() unexpected error: %v", err)
	}

	if got := obj.GetAnnotations()[declarestv1alpha1.CRDGeneratorOwnerAnnotation]; got != "operators/realms" {
		t.Fatalf("expected owner annotation operators/realms, got %q", got)
	}
	if !containsString(obj.GetFinalizers(), finalizerName) {
		t.Fatalf("expected generated resource finalizer %q, got %#v", finalizerName, obj.GetFinalizers())
	}
}

func TestGeneratedCRDSchemaDefaultsDeletionPolicyToOrphan(t *testing.T) {
	t.Parallel()

	schemaProps := generatedCRDSchema("/admin/realms")
	specSchema := schemaProps.Properties["spec"]
	deletionPolicy := specSchema.Properties["deletionPolicy"]
	if deletionPolicy.Default == nil || string(deletionPolicy.Default.Raw) != `"Orphan"` {
		t.Fatalf("expected deletionPolicy default Orphan, got %#v", deletionPolicy.Default)
	}
	wantEnum := map[string]struct{}{
		`"Orphan"`: {},
		`"Delete"`: {},
	}
	for _, item := range deletionPolicy.Enum {
		delete(wantEnum, string(item.Raw))
	}
	if len(wantEnum) != 0 {
		t.Fatalf("deletionPolicy enum is missing values: %#v", wantEnum)
	}
	if status := schemaProps.Properties["status"]; status.Type != "object" {
		t.Fatalf("expected generated CRD status schema, got %#v", status)
	}
}

func TestGeneratedResourcePathsUseOverride(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	obj.Object = map[string]any{
		"spec": map[string]any{
			"collectionPathOverride": "admin/overrides",
		},
	}
	obj.SetName("acme")
	collection, logical, err := generatedResourcePaths(&declarestv1alpha1.CRDGeneratorVersion{CollectionPath: "/admin/realms"}, obj)
	if err != nil {
		t.Fatalf("generatedResourcePaths() unexpected error: %v", err)
	}
	if collection != "/admin/overrides" || logical != "/admin/overrides/acme" {
		t.Fatalf("unexpected generated paths: collection=%q logical=%q", collection, logical)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
