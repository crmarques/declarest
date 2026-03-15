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

	corev1 "k8s.io/api/core/v1"
)

func TestSecretStoreValidateSpecFile(t *testing.T) {
	t.Parallel()

	store := &SecretStore{
		Spec: SecretStoreSpec{
			File: &SecretStoreFileSpec{
				Path: "/var/lib/declarest/secrets/secrets.json",
				Storage: StorageSpec{
					ExistingPVC: &corev1.LocalObjectReference{Name: "secret-store-pvc"},
				},
				Encryption: SecretStoreFileEncryption{
					PassphraseRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-store-file"},
						Key:                  "passphrase",
					},
				},
			},
		},
	}

	if err := store.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestSecretStoreValidateSpecVaultUserpass(t *testing.T) {
	t.Parallel()

	store := &SecretStore{
		Spec: SecretStoreSpec{
			Vault: &SecretStoreVaultSpec{
				Address: "https://vault.example.com",
				Auth: SecretStoreVaultAuth{
					Userpass: &SecretStoreVaultUserpassAuth{
						UsernameRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "username",
						},
						PasswordRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "password",
						},
						Mount: "userpass",
					},
				},
			},
		},
	}

	if err := store.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestSecretStoreValidateSpecAllowsSparseProxyOverride(t *testing.T) {
	t.Parallel()

	store := &SecretStore{
		Spec: SecretStoreSpec{
			Vault: &SecretStoreVaultSpec{
				Address: "https://vault.example.com",
				Auth: SecretStoreVaultAuth{
					Userpass: &SecretStoreVaultUserpassAuth{
						UsernameRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "username",
						},
						PasswordRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vault-auth"},
							Key:                  "password",
						},
					},
				},
				Proxy: &HTTPProxySpec{
					NoProxy: "localhost,127.0.0.1",
				},
			},
		},
	}

	if err := store.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error: %v", err)
	}
}

func TestSecretStoreValidateSpecRejectsMultipleBackends(t *testing.T) {
	t.Parallel()

	store := &SecretStore{
		Spec: SecretStoreSpec{
			File: &SecretStoreFileSpec{
				Path: "/var/lib/declarest/secrets/secrets.json",
				Storage: StorageSpec{
					ExistingPVC: &corev1.LocalObjectReference{Name: "secret-store-pvc"},
				},
				Encryption: SecretStoreFileEncryption{
					PassphraseRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-store-file"},
						Key:                  "passphrase",
					},
				},
			},
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

	if err := store.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected backend one-of error, got nil")
	}
}
