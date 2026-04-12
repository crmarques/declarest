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
	"os"
	"path/filepath"
	"testing"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// envTestState holds shared state for envtest-based integration tests.
type envTestState struct {
	env    *envtest.Environment
	client client.Client
	cancel context.CancelFunc
}

func stopEnvTest(t *testing.T, testEnv *envtest.Environment) {
	t.Helper()
	if testEnv == nil {
		return
	}
	if err := testEnv.Stop(); err != nil {
		t.Errorf("stop envtest: %v", err)
	}
}

// setupEnvTest starts a real API server with the DeclaREST CRDs installed and
// returns a client for interacting with it. Call teardown() when done.
//
// Tests guarded by DECLAREST_ENVTEST=1 are skipped by default because envtest
// requires etcd and kube-apiserver binaries available via setup-envtest.
func setupEnvTest(t *testing.T) *envTestState {
	t.Helper()

	if os.Getenv("DECLAREST_ENVTEST") != "1" {
		t.Skip("set DECLAREST_ENVTEST=1 to run envtest integration tests")
	}

	scheme := runtime.NewScheme()
	if err := k8sscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add k8s scheme: %v", err)
	}
	if err := declarestv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add declarest scheme: %v", err)
	}

	crdPath := filepath.Join("..", "..", "..", "config", "crd", "bases")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{crdPath},
		Scheme:            scheme,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		stopEnvTest(t, testEnv)
		t.Fatalf("create client: %v", err)
	}

	// Create the test namespace.
	ctx, cancel := context.WithCancel(context.Background())
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	if err := k8sClient.Create(ctx, ns); err != nil {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatalf("create test namespace: %v", err)
	}

	// Start the manager with controllers.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatalf("create manager: %v", err)
	}

	if err := (&ManagedServiceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("managedservice-controller"),
	}).SetupWithManager(mgr); err != nil {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatalf("setup ManagedService controller: %v", err)
	}
	if err := (&SecretStoreReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("secretstore-controller"),
	}).SetupWithManager(mgr); err != nil {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatalf("setup SecretStore controller: %v", err)
	}
	if err := (&RepositoryWebhookReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorder("repositorywebhook-controller"),
	}).SetupWithManager(mgr); err != nil {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatalf("setup RepositoryWebhook controller: %v", err)
	}

	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager stopped: %v", err)
		}
	}()

	// Wait for the cache to sync.
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		cancel()
		stopEnvTest(t, testEnv)
		t.Fatal("cache sync failed")
	}

	t.Cleanup(func() {
		cancel()
		stopEnvTest(t, testEnv)
	})

	return &envTestState{
		env:    testEnv,
		client: k8sClient,
		cancel: cancel,
	}
}

// waitForCondition polls a resource until the named condition matches the
// expected status or the timeout elapses.
func waitForCondition(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	key types.NamespacedName,
	obj client.Object,
	conditionType string,
	expectedStatus metav1.ConditionStatus,
) {
	t.Helper()

	deadline := time.After(15 * time.Second)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for condition %s=%s on %s/%s", conditionType, expectedStatus, key.Namespace, key.Name)
		case <-ticker.C:
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				continue
			}
			conditions := extractConditions(obj)
			for _, c := range conditions {
				if c.Type == conditionType && c.Status == expectedStatus {
					return
				}
			}
		}
	}
}

// extractConditions returns the conditions from a DeclaREST resource.
func extractConditions(obj client.Object) []metav1.Condition {
	switch v := obj.(type) {
	case *declarestv1alpha1.ManagedService:
		return v.Status.Conditions
	case *declarestv1alpha1.SecretStore:
		return v.Status.Conditions
	case *declarestv1alpha1.ResourceRepository:
		return v.Status.Conditions
	case *declarestv1alpha1.SyncPolicy:
		return v.Status.Conditions
	case *declarestv1alpha1.RepositoryWebhook:
		return v.Status.Conditions
	default:
		return nil
	}
}
