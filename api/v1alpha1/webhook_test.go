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
					TokenRef: &corev1.SecretKeySelector{
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

	v := &resourceRepositoryValidator{}
	warnings, err := v.ValidateCreate(context.Background(), repo.DeepCopy())
	if err != nil {
		t.Fatalf("ValidateCreate() unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ValidateCreate() returned unexpected warnings: %v", warnings)
	}
}

func TestWebhookValidationSecretStoreCreate(t *testing.T) {
	t.Parallel()

	secretStore := &SecretStore{
		Spec: SecretStoreSpec{
			Vault: &SecretStoreVaultSpec{
				Address: "https://vault.example.com",
				Auth: SecretStoreVaultAuth{
					Token: &SecretStoreVaultTokenAuth{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "token",
						},
					},
				},
			},
		},
	}

	v := &secretStoreValidator{}
	warnings, err := v.ValidateCreate(context.Background(), secretStore.DeepCopy())
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

	v := &managedServerValidator{}
	if _, err := v.ValidateCreate(context.Background(), server.DeepCopy()); err == nil {
		t.Fatal("ValidateCreate() expected validation error, got nil")
	}
}
