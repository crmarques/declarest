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
