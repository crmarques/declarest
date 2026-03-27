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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
					TokenRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
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
					TokenRef:     &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
					SSHSecretRef: &GitSSHSecretRef{PrivateKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "ssh"}, Key: "privateKey"}},
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
					TokenRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
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

func TestResourceRepositoryValidateSpecRejectsMissingPVCAccessModes(t *testing.T) {
	t.Parallel()

	repo := &ResourceRepository{
		Spec: ResourceRepositorySpec{
			Type:         ResourceRepositoryTypeGit,
			PollInterval: metav1.Duration{Duration: 30 * time.Second},
			Git: &GitRepositorySpec{
				URL:    "https://example.com/org/repo.git",
				Branch: "main",
				Auth: ResourceRepositoryAuth{
					TokenRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "git-auth"}, Key: "token"},
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

	if err := repo.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected pvc accessModes validation error, got nil")
	}
}
