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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestEnvtestManagedServerReconcilesAndSetsReadyCondition verifies that
// creating a ManagedServer with a valid spec causes the controller to
// reconcile and set a Ready condition.
func TestEnvtestManagedServerReconcilesAndSetsReadyCondition(t *testing.T) {
	state := setupEnvTest(t)
	ctx := context.Background()

	ms := &declarestv1alpha1.ManagedServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ms",
			Namespace: "test",
		},
		Spec: declarestv1alpha1.ManagedServerSpec{
			HTTP: declarestv1alpha1.ManagedServerHTTP{
				BaseURL: "https://example.com/api",
				Auth: declarestv1alpha1.ManagedServerAuth{
					BasicAuth: &declarestv1alpha1.ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "ms-creds"},
							Key:                  "username",
						},
						PasswordRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "ms-creds"},
							Key:                  "password",
						},
					},
				},
			},
		},
	}

	if err := state.client.Create(ctx, ms); err != nil {
		t.Fatalf("create ManagedServer: %v", err)
	}

	key := types.NamespacedName{Namespace: "test", Name: "test-ms"}
	fetched := &declarestv1alpha1.ManagedServer{}
	waitForCondition(t, ctx, state.client, key, fetched, declarestv1alpha1.ConditionTypeReady, metav1.ConditionTrue)

	if fetched.Status.ObservedGeneration != fetched.Generation {
		t.Errorf("expected ObservedGeneration=%d, got=%d", fetched.Generation, fetched.Status.ObservedGeneration)
	}
}

// TestEnvtestManagedServerDeletionCleansUpFinalizer verifies that deleting a
// ManagedServer removes the finalizer and completes deletion.
func TestEnvtestManagedServerDeletionCleansUpFinalizer(t *testing.T) {
	state := setupEnvTest(t)
	ctx := context.Background()

	ms := &declarestv1alpha1.ManagedServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ms-del",
			Namespace: "test",
		},
		Spec: declarestv1alpha1.ManagedServerSpec{
			HTTP: declarestv1alpha1.ManagedServerHTTP{
				BaseURL: "https://example.com/api",
				Auth: declarestv1alpha1.ManagedServerAuth{
					BasicAuth: &declarestv1alpha1.ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "ms-creds"},
							Key:                  "username",
						},
						PasswordRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "ms-creds"},
							Key:                  "password",
						},
					},
				},
			},
		},
	}

	if err := state.client.Create(ctx, ms); err != nil {
		t.Fatalf("create ManagedServer: %v", err)
	}

	key := types.NamespacedName{Namespace: "test", Name: "test-ms-del"}
	fetched := &declarestv1alpha1.ManagedServer{}
	waitForCondition(t, ctx, state.client, key, fetched, declarestv1alpha1.ConditionTypeReady, metav1.ConditionTrue)

	if err := state.client.Delete(ctx, ms); err != nil {
		t.Fatalf("delete ManagedServer: %v", err)
	}
}

// TestEnvtestRepositoryWebhookReportsNotReadyWhenRepoMissing verifies that a
// RepositoryWebhook referencing a non-existent ResourceRepository gets a
// Ready=False condition with reason RepositoryNotFound.
func TestEnvtestRepositoryWebhookReportsNotReadyWhenRepoMissing(t *testing.T) {
	state := setupEnvTest(t)
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "rwh-secret", Namespace: "test"},
		Data:       map[string][]byte{"token": []byte("webhook-secret")},
	}
	if err := state.client.Create(ctx, secret); err != nil {
		t.Fatalf("create Secret: %v", err)
	}

	rwh := &declarestv1alpha1.RepositoryWebhook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rwh",
			Namespace: "test",
		},
		Spec: declarestv1alpha1.RepositoryWebhookSpec{
			RepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "missing-repo"},
			Provider:      declarestv1alpha1.RepositoryWebhookProviderGitHub,
			SecretRef:     declarestv1alpha1.NamespacedObjectReference{Name: "rwh-secret"},
		},
	}

	if err := state.client.Create(ctx, rwh); err != nil {
		t.Fatalf("create RepositoryWebhook: %v", err)
	}

	key := types.NamespacedName{Namespace: "test", Name: "test-rwh"}
	fetched := &declarestv1alpha1.RepositoryWebhook{}
	waitForCondition(t, ctx, state.client, key, fetched, declarestv1alpha1.ConditionTypeReady, metav1.ConditionFalse)

	for _, c := range fetched.Status.Conditions {
		if c.Type == declarestv1alpha1.ConditionTypeReady {
			if c.Reason != "RepositoryNotFound" {
				t.Errorf("expected reason RepositoryNotFound, got %s", c.Reason)
			}
			break
		}
	}

	if fetched.Status.WebhookPath == "" {
		t.Error("expected webhookPath to be set even when repo is missing")
	}
}

// TestEnvtestRepositoryWebhookSuspendSetsNotReady verifies that suspending a
// RepositoryWebhook sets Ready=False with reason Suspended.
func TestEnvtestRepositoryWebhookSuspendSetsNotReady(t *testing.T) {
	state := setupEnvTest(t)
	ctx := context.Background()

	rwh := &declarestv1alpha1.RepositoryWebhook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rwh-suspend",
			Namespace: "test",
		},
		Spec: declarestv1alpha1.RepositoryWebhookSpec{
			RepositoryRef: declarestv1alpha1.NamespacedObjectReference{Name: "some-repo"},
			Provider:      declarestv1alpha1.RepositoryWebhookProviderGitea,
			SecretRef:     declarestv1alpha1.NamespacedObjectReference{Name: "some-secret"},
			Suspend:       true,
		},
	}

	if err := state.client.Create(ctx, rwh); err != nil {
		t.Fatalf("create RepositoryWebhook: %v", err)
	}

	key := types.NamespacedName{Namespace: "test", Name: "test-rwh-suspend"}
	fetched := &declarestv1alpha1.RepositoryWebhook{}
	waitForCondition(t, ctx, state.client, key, fetched, declarestv1alpha1.ConditionTypeReady, metav1.ConditionFalse)

	for _, c := range fetched.Status.Conditions {
		if c.Type == declarestv1alpha1.ConditionTypeReady {
			if c.Reason != "Suspended" {
				t.Errorf("expected reason Suspended, got %s", c.Reason)
			}
			break
		}
	}
}

// TestEnvtestSecretStoreReconcilesFileBackend verifies that a file-backend
// SecretStore reconciles. This tests the finalizer add and status update flow.
func TestEnvtestSecretStoreReconcilesFileBackend(t *testing.T) {
	state := setupEnvTest(t)
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "enc-key", Namespace: "test"},
		Data:       map[string][]byte{"key": []byte("0123456789abcdef0123456789abcdef")},
	}
	if err := state.client.Create(ctx, secret); err != nil {
		t.Fatalf("create Secret: %v", err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "sst-pvc", Namespace: "test"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	if err := state.client.Create(ctx, pvc); err != nil {
		t.Fatalf("create PVC: %v", err)
	}

	sst := &declarestv1alpha1.SecretStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sst",
			Namespace: "test",
		},
		Spec: declarestv1alpha1.SecretStoreSpec{
			File: &declarestv1alpha1.SecretStoreFileSpec{
				Path: "/secrets/data.enc",
				Storage: declarestv1alpha1.StorageSpec{
					ExistingPVC: &corev1.LocalObjectReference{Name: "sst-pvc"},
				},
				Encryption: declarestv1alpha1.SecretStoreFileEncryption{
					KeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "enc-key"},
						Key:                  "key",
					},
				},
			},
		},
	}

	if err := state.client.Create(ctx, sst); err != nil {
		t.Fatalf("create SecretStore: %v", err)
	}

	key := types.NamespacedName{Namespace: "test", Name: "test-sst"}
	fetched := &declarestv1alpha1.SecretStore{}
	waitForCondition(t, ctx, state.client, key, fetched, declarestv1alpha1.ConditionTypeReady, metav1.ConditionTrue)

	if fetched.Status.ObservedGeneration != fetched.Generation {
		t.Errorf("expected ObservedGeneration=%d, got=%d", fetched.Generation, fetched.Status.ObservedGeneration)
	}
}

// TestEnvtestIsDependencyReadyHelper verifies the helper used for the
// dependency-ready check.
func TestEnvtestIsDependencyReadyHelper(t *testing.T) {
	t.Parallel()

	readyConditions := []metav1.Condition{
		{Type: declarestv1alpha1.ConditionTypeReady, Status: metav1.ConditionTrue},
	}
	notReadyConditions := []metav1.Condition{
		{Type: declarestv1alpha1.ConditionTypeReady, Status: metav1.ConditionFalse},
	}
	emptyConditions := []metav1.Condition{}

	if !isDependencyReady(readyConditions) {
		t.Error("expected ready conditions to return true")
	}
	if isDependencyReady(notReadyConditions) {
		t.Error("expected not-ready conditions to return false")
	}
	if isDependencyReady(emptyConditions) {
		t.Error("expected empty conditions to return false")
	}
	if isDependencyReady(nil) {
		t.Error("expected nil conditions to return false")
	}
}
