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
	if policy.Spec.Sync.Force {
		t.Fatalf("expected sync.force default false, got true")
	}
}

func TestSyncPolicyValidateSpecAcceptsValidFullResyncCron(t *testing.T) {
	t.Parallel()

	policy := &SyncPolicy{
		Spec: SyncPolicySpec{
			ResourceRepositoryRef: NamespacedObjectReference{Name: "repo"},
			ManagedServerRef:      NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        NamespacedObjectReference{Name: "secrets"},
			Source:                SyncPolicySource{Path: "/customers"},
			FullResyncCron:        "*/30 * * * *",
		},
	}

	if err := policy.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestSyncPolicyValidateSpecRejectsInvalidFullResyncCron(t *testing.T) {
	t.Parallel()

	policy := &SyncPolicy{
		Spec: SyncPolicySpec{
			ResourceRepositoryRef: NamespacedObjectReference{Name: "repo"},
			ManagedServerRef:      NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        NamespacedObjectReference{Name: "secrets"},
			Source:                SyncPolicySource{Path: "/customers"},
			FullResyncCron:        "invalid-cron",
		},
	}

	if err := policy.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected fullResyncCron validation error, got nil")
	}
}

func TestSyncPolicyValidateSpecRejectsTraversalPath(t *testing.T) {
	t.Parallel()

	policy := &SyncPolicy{
		Spec: SyncPolicySpec{
			ResourceRepositoryRef: NamespacedObjectReference{Name: "repo"},
			ManagedServerRef:      NamespacedObjectReference{Name: "server"},
			SecretStoreRef:        NamespacedObjectReference{Name: "secrets"},
			Source:                SyncPolicySource{Path: "/customers/../acme"},
		},
	}

	if err := policy.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected traversal validation error, got nil")
	}
}
