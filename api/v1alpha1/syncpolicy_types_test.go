package v1alpha1

import "testing"

func TestSyncPolicyValidateSpecNormalizesPathAndDefaultsRecursive(t *testing.T) {
	t.Parallel()

	policy := &SyncPolicy{
		Spec: SyncPolicySpec{
			ResourceRepositoryRef: NamespacedObjectReference{Name: "repo"},
			ManagedServerRef:      NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        NamespacedObjectReference{Name: "secrets"},
			Source:                SyncPolicySource{Path: "customers/acme"},
		},
	}

	if err := policy.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
	if policy.Spec.Source.Path != "/customers/acme" {
		t.Fatalf("expected normalized path /customers/acme, got %q", policy.Spec.Source.Path)
	}
	if policy.Spec.Source.Recursive == nil || !*policy.Spec.Source.Recursive {
		t.Fatalf("expected recursive default true, got %v", policy.Spec.Source.Recursive)
	}
}
