package v1alpha1

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceRepositoryValidateSpec(t *testing.T) {
	t.Parallel()

	repo := &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: ResourceRepositoryAuth{
					TokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
				},
			},
			Storage: StorageSpec{
				ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"},
			},
		},
	}

	if err := repo.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestResourceRepositoryValidateSpecAuthOneOf(t *testing.T) {
	t.Parallel()

	repo := &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: ResourceRepositoryAuth{
					TokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
					SSHSecretRef:   &GitSSHSecretRef{PrivateKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "ssh"}, Key: "privateKey"}},
				},
			},
			Storage: StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"}},
		},
	}

	if err := repo.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected one-of auth error, got nil")
	}
}

func TestResourceRepositoryValidateSpecWebhook(t *testing.T) {
	t.Parallel()

	repo := &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: ResourceRepositoryAuth{
					TokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
				},
				Webhook: &GitRepositoryWebhookSpec{
					Provider:  GitWebhookProviderGitea,
					SecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-webhook"}, Key: "secret"},
				},
			},
			Storage: StorageSpec{ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"}},
		},
	}

	if err := repo.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error for valid webhook: %v", err)
	}

	repo.Spec.Git.Webhook.Provider = "unknown"
	if err := repo.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected webhook provider validation error, got nil")
	}
}
