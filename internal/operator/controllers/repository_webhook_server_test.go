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

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
					TokenSecretRef: &corev1.SecretKeySelector{
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
					TokenSecretRef: &corev1.SecretKeySelector{
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
