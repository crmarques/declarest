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
		},
	}

	names := collectSecretNames(repo, &declarestv1alpha1.ManagedServer{}, &declarestv1alpha1.SecretStore{})
	if len(names) != 2 {
		t.Fatalf("expected 2 secret names, got %#v", names)
	}
	if names[0] != "git-auth" || names[1] != "repo-webhook" {
		t.Fatalf("unexpected secret names: %#v", names)
	}
}
