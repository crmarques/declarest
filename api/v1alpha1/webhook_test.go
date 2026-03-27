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

package v1alpha1

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestWebhookValidationResourceRepositoryCreateRejectsMissingPVCAccessModes(t *testing.T) {
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
				PVC: &PVCTemplateSpec{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	v := &resourceRepositoryValidator{}
	if _, err := v.ValidateCreate(context.Background(), repo.DeepCopy()); err == nil {
		t.Fatal("ValidateCreate() expected pvc accessModes validation error, got nil")
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

func TestWebhookValidationAcceptsEnvPlaceholders(t *testing.T) {
	t.Setenv("DECLAREST_WEBHOOK_REPO_URL", "https://example.com/org/runtime.git")
	t.Setenv("DECLAREST_WEBHOOK_SERVER_URL", "https://runtime.example.com/api")
	t.Setenv("DECLAREST_WEBHOOK_OPENAPI_URL", "https://runtime.example.com/openapi.json")
	t.Setenv("DECLAREST_WEBHOOK_VAULT_URL", "https://vault.runtime.example.com")
	t.Setenv("DECLAREST_WEBHOOK_SYNC_PATH", "/customers/runtime")

	repoValidator := &resourceRepositoryValidator{}
	if _, err := repoValidator.ValidateCreate(context.Background(), &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "${DECLAREST_WEBHOOK_REPO_URL}",
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
	}); err != nil {
		t.Fatalf("resource repository ValidateCreate() unexpected error: %v", err)
	}

	serverValidator := &managedServerValidator{}
	if _, err := serverValidator.ValidateCreate(context.Background(), &ManagedServer{
		Spec: ManagedServerSpec{
			HTTP: ManagedServerHTTP{
				BaseURL: "${DECLAREST_WEBHOOK_SERVER_URL}",
				Auth: ManagedServerAuth{
					OAuth2: &ManagedServerOAuth2Auth{
						TokenURL:  "${DECLAREST_WEBHOOK_OPENAPI_URL}",
						GrantType: "client_credentials",
						ClientIDRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "oauth"},
							Key:                  "client-id",
						},
						ClientSecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "oauth"},
							Key:                  "client-secret",
						},
					},
				},
			},
			OpenAPI: DeclaRESTExternalArtifact{URL: "${DECLAREST_WEBHOOK_OPENAPI_URL}"},
		},
	}); err != nil {
		t.Fatalf("managed server ValidateCreate() unexpected error: %v", err)
	}

	secretStoreValidator := &secretStoreValidator{}
	if _, err := secretStoreValidator.ValidateCreate(context.Background(), &SecretStore{
		Spec: SecretStoreSpec{
			Vault: &SecretStoreVaultSpec{
				Address: "${DECLAREST_WEBHOOK_VAULT_URL}",
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
	}); err != nil {
		t.Fatalf("secret store ValidateCreate() unexpected error: %v", err)
	}

	syncPolicyValidator := &syncPolicyValidator{Client: fakeSyncPolicyClient{}}
	if _, err := syncPolicyValidator.ValidateCreate(context.Background(), &SyncPolicy{
		Spec: SyncPolicySpec{
			ResourceRepositoryRef: NamespacedObjectReference{Name: "repo"},
			ManagedServerRef:      NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        NamespacedObjectReference{Name: "secret-store"},
			Source:                SyncPolicySource{Path: "${DECLAREST_WEBHOOK_SYNC_PATH}"},
		},
	}); err != nil {
		t.Fatalf("sync policy ValidateCreate() unexpected error: %v", err)
	}
}

type fakeSyncPolicyClient struct{}

func (fakeSyncPolicyClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return nil
}

func (fakeSyncPolicyClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	if syncPolicies, ok := list.(*SyncPolicyList); ok {
		syncPolicies.Items = nil
	}
	return nil
}
