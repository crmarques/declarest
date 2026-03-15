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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRepositoryWebhookServerBuildsHTTPServerWithDefaults(t *testing.T) {
	t.Parallel()

	server := (&RepositoryWebhookServer{}).buildHTTPServer(":9443", http.NewServeMux())
	if server.ReadHeaderTimeout != defaultWebhookReadHeaderTimeout {
		t.Fatalf("expected default read header timeout %s, got %s", defaultWebhookReadHeaderTimeout, server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != defaultWebhookReadTimeout {
		t.Fatalf("expected default read timeout %s, got %s", defaultWebhookReadTimeout, server.ReadTimeout)
	}
	if server.WriteTimeout != defaultWebhookWriteTimeout {
		t.Fatalf("expected default write timeout %s, got %s", defaultWebhookWriteTimeout, server.WriteTimeout)
	}
	if server.IdleTimeout != defaultWebhookIdleTimeout {
		t.Fatalf("expected default idle timeout %s, got %s", defaultWebhookIdleTimeout, server.IdleTimeout)
	}
}

func TestRepositoryWebhookServerBuildsHTTPServerWithCustomTimeouts(t *testing.T) {
	t.Parallel()

	sut := &RepositoryWebhookServer{
		ReadHeaderLimit: 2 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    4 * time.Second,
		IdleTimeout:     5 * time.Second,
	}
	server := sut.buildHTTPServer(":9443", http.NewServeMux())
	if server.ReadHeaderTimeout != sut.ReadHeaderLimit {
		t.Fatalf("expected custom read header timeout %s, got %s", sut.ReadHeaderLimit, server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != sut.ReadTimeout {
		t.Fatalf("expected custom read timeout %s, got %s", sut.ReadTimeout, server.ReadTimeout)
	}
	if server.WriteTimeout != sut.WriteTimeout {
		t.Fatalf("expected custom write timeout %s, got %s", sut.WriteTimeout, server.WriteTimeout)
	}
	if server.IdleTimeout != sut.IdleTimeout {
		t.Fatalf("expected custom idle timeout %s, got %s", sut.IdleTimeout, server.IdleTimeout)
	}
}

func TestRepositoryWebhookServerAcceptsValidGiteaWebhook(t *testing.T) {
	t.Parallel()

	const (
		namespace  = "default"
		repoName   = "repo"
		secretName = "repo-webhook"
		secretKey  = "secret"
		secretVal  = "super-secret"
	)
	payload := []byte(`{"ref":"refs/heads/main"}`)
	signature := hmac.New(sha256.New, []byte(secretVal))
	_, _ = signature.Write(payload)
	expectedSignature := hex.EncodeToString(signature.Sum(nil))

	scheme := runtime.NewScheme()
	_ = declarestv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	repo := &declarestv1alpha1.ResourceRepository{
		ObjectMeta: metav1.ObjectMeta{Name: repoName, Namespace: namespace},
		Spec: declarestv1alpha1.ResourceRepositorySpec{
			Type:         declarestv1alpha1.ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30_000_000_000},
			Git: &declarestv1alpha1.GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: declarestv1alpha1.ResourceRepositoryAuth{
					TokenRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"},
						Key:                  "token",
					},
				},
				Webhook: &declarestv1alpha1.GitRepositoryWebhookSpec{
					Provider: declarestv1alpha1.GitWebhookProviderGitea,
					SecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  secretKey,
					},
				},
			},
			Storage: declarestv1alpha1.StorageSpec{
				ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"},
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
		Data: map[string][]byte{
			secretKey: []byte(secretVal),
			"token":   []byte("git-token"),
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo, secret).Build()
	server := &RepositoryWebhookServer{Client: cl}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/repository/default/repo", bytes.NewReader(payload))
	req.Header.Set("X-Gitea-Signature", expectedSignature)
	req.Header.Set("X-Gitea-Event", "push")
	req.Header.Set("X-Gitea-Delivery", "delivery-id")
	rec := httptest.NewRecorder()

	server.handleRepositoryWebhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d body=%q", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	updated := &declarestv1alpha1.ResourceRepository{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: repoName}, updated); err != nil {
		t.Fatalf("failed to fetch updated repository: %v", err)
	}
	if updated.Annotations[repositoryWebhookAnnotationLastEventAt] == "" {
		t.Fatal("expected webhook receipt annotation to be set")
	}
	if updated.Annotations[repositoryWebhookAnnotationLastEventID] != "delivery-id" {
		t.Fatalf("expected webhook event id annotation, got %q", updated.Annotations[repositoryWebhookAnnotationLastEventID])
	}
}

func TestRepositoryWebhookServerRejectsInvalidSignature(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = declarestv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	repo := &declarestv1alpha1.ResourceRepository{
		ObjectMeta: metav1.ObjectMeta{Name: "repo", Namespace: "default"},
		Spec: declarestv1alpha1.ResourceRepositorySpec{
			Type:         declarestv1alpha1.ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30_000_000_000},
			Git: &declarestv1alpha1.GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: declarestv1alpha1.ResourceRepositoryAuth{
					TokenRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"},
						Key:                  "token",
					},
				},
				Webhook: &declarestv1alpha1.GitRepositoryWebhookSpec{
					Provider: declarestv1alpha1.GitWebhookProviderGitea,
					SecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "repo-webhook"},
						Key:                  "secret",
					},
				},
			},
			Storage: declarestv1alpha1.StorageSpec{
				ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"},
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "repo-webhook", Namespace: "default"},
		Data: map[string][]byte{
			"secret": []byte("correct-secret"),
			"token":  []byte("git-token"),
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(repo, secret).Build()
	server := &RepositoryWebhookServer{Client: cl}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/repository/default/repo", bytes.NewReader([]byte(`{"ref":"refs/heads/main"}`)))
	req.Header.Set("X-Gitea-Signature", "deadbeef")
	req.Header.Set("X-Gitea-Event", "push")
	rec := httptest.NewRecorder()

	server.handleRepositoryWebhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%q", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	updated := &declarestv1alpha1.ResourceRepository{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "repo"}, updated); err != nil {
		t.Fatalf("failed to fetch updated repository: %v", err)
	}
	if updated.Annotations != nil && updated.Annotations[repositoryWebhookAnnotationLastEventAt] != "" {
		t.Fatal("expected no webhook receipt annotation on authentication failure")
	}
}
