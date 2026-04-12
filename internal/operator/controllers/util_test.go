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
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestHasPathOverlap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  string
		right string
		match bool
	}{
		{name: "same path", left: "/customers/acme", right: "/customers/acme", match: true},
		{name: "parent child", left: "/customers", right: "/customers/acme", match: true},
		{name: "sibling", left: "/customers/acme", right: "/customers/beta", match: false},
		{name: "root overlap", left: "/", right: "/customers", match: true},
		{name: "relative path normalized", left: "customers", right: "/customers/acme", match: true},
		{name: "invalid traversal rejected", left: "/customers/../acme", right: "/customers", match: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := hasPathOverlap(tc.left, tc.right); got != tc.match {
				t.Fatalf("hasPathOverlap(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.match)
			}
		})
	}
}

func TestCollectSecretNamesIncludesRepositoryWebhookSecret(t *testing.T) {
	t.Parallel()

	repo := &declarestv1alpha1.ResourceRepository{
		Spec: declarestv1alpha1.ResourceRepositorySpec{
			Git: &declarestv1alpha1.GitRepositorySpec{
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
		},
	}

	secretStore := &declarestv1alpha1.SecretStore{
		Spec: declarestv1alpha1.SecretStoreSpec{
			Vault: &declarestv1alpha1.SecretStoreVaultSpec{
				Address: "https://vault.example.com",
				Auth: declarestv1alpha1.SecretStoreVaultAuth{
					Token: &declarestv1alpha1.SecretStoreVaultTokenAuth{
						SecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "token",
						},
					},
				},
			},
		},
	}

	names := collectSecretNames(repo, &declarestv1alpha1.ManagedService{}, secretStore)
	if len(names) != 3 {
		t.Fatalf("expected 2 secret names, got %#v", names)
	}
	if names[0] != "git-auth" || names[1] != "repo-webhook" || names[2] != "vault-auth" {
		t.Fatalf("unexpected secret names: %#v", names)
	}
}

func TestExpandRuntimeSpecsResolveEnvPlaceholders(t *testing.T) {
	t.Setenv("DECLAREST_REPO_URL", "https://example.com/org/runtime.git")
	t.Setenv("DECLAREST_SERVER_URL", "https://runtime.example.com/api")
	t.Setenv("DECLAREST_OPENAPI_URL", "https://runtime.example.com/openapi.json")
	t.Setenv("DECLAREST_SECRET_PATH", "/data/runtime/secrets.json")
	t.Setenv("DECLAREST_SYNC_PATH", "/customers/runtime")

	repo := expandRuntimeResourceRepository(&declarestv1alpha1.ResourceRepository{
		Spec: declarestv1alpha1.ResourceRepositorySpec{
			Git: &declarestv1alpha1.GitRepositorySpec{
				URL: "${DECLAREST_REPO_URL}",
			},
		},
	})
	if got := repo.Spec.Git.URL; got != "https://example.com/org/runtime.git" {
		t.Fatalf("expected expanded repository URL, got %q", got)
	}

	server := expandRuntimeManagedService(&declarestv1alpha1.ManagedService{
		Spec: declarestv1alpha1.ManagedServiceSpec{
			HTTP: declarestv1alpha1.ManagedServiceHTTP{
				BaseURL: "${DECLAREST_SERVER_URL}",
			},
			OpenAPI: declarestv1alpha1.DeclaRESTExternalArtifact{
				URL: "${DECLAREST_OPENAPI_URL}",
			},
		},
	})
	if got := server.Spec.HTTP.BaseURL; got != "https://runtime.example.com/api" {
		t.Fatalf("expected expanded managed-service baseURL, got %q", got)
	}
	if got := server.Spec.OpenAPI.URL; got != "https://runtime.example.com/openapi.json" {
		t.Fatalf("expected expanded openapi URL, got %q", got)
	}

	secretStore := expandRuntimeSecretStore(&declarestv1alpha1.SecretStore{
		Spec: declarestv1alpha1.SecretStoreSpec{
			File: &declarestv1alpha1.SecretStoreFileSpec{
				Path: "${DECLAREST_SECRET_PATH}",
			},
		},
	})
	if got := secretStore.Spec.File.Path; got != "/data/runtime/secrets.json" {
		t.Fatalf("expected expanded secret-store path, got %q", got)
	}

	syncPolicy := expandRuntimeSyncPolicy(&declarestv1alpha1.SyncPolicy{
		Spec: declarestv1alpha1.SyncPolicySpec{
			Source: declarestv1alpha1.SyncPolicySource{
				Path: "${DECLAREST_SYNC_PATH}",
			},
		},
	})
	if got := syncPolicy.Spec.Source.Path; got != "/customers/runtime" {
		t.Fatalf("expected expanded sync-policy path, got %q", got)
	}
}
