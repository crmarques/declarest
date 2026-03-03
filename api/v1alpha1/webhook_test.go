package v1alpha1

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWebhookValidationResourceRepositoryCreate(t *testing.T) {
	t.Parallel()

	repo := &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: ResourceRepositoryAuth{
					TokenSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"},
						Key:                  "token",
					},
				},
			},
			Storage: StorageSpec{
				ExistingPVC: &corev1.LocalObjectReference{Name: "repo-pvc"},
			},
		},
	}

	warnings, err := repo.ValidateCreate(context.Background(), repo.DeepCopy())
	if err != nil {
		t.Fatalf("ValidateCreate() unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ValidateCreate() returned unexpected warnings: %v", warnings)
	}
}

func TestWebhookValidationManagedServerCreateRejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	server := &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "not-a-url",
				Auth: ManagedServerAuth{
					BasicAuth: &ManagedServerBasicAuth{
						UsernameRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "auth"}, Key: "username"},
						PasswordRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "auth"}, Key: "password"},
					},
				},
			},
		},
	}

	if _, err := server.ValidateCreate(context.Background(), server.DeepCopy()); err == nil {
		t.Fatal("ValidateCreate() expected validation error, got nil")
	}
}
