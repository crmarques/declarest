package v1alpha1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretStoreVaultTokenAuth struct {
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

type SecretStoreVaultUserpassAuth struct {
	UsernameRef *corev1.SecretKeySelector `json:"usernameRef,omitempty"`
	PasswordRef *corev1.SecretKeySelector `json:"passwordRef,omitempty"`
	Mount       string                    `json:"mount,omitempty"`
}

type SecretStoreVaultAppRoleAuth struct {
	RoleIDRef   *corev1.SecretKeySelector `json:"roleIDRef,omitempty"`
	SecretIDRef *corev1.SecretKeySelector `json:"secretIDRef,omitempty"`
	Mount       string                    `json:"mount,omitempty"`
}

type SecretStoreVaultAuth struct {
	Token    *SecretStoreVaultTokenAuth    `json:"token,omitempty"`
	Userpass *SecretStoreVaultUserpassAuth `json:"userpass,omitempty"`
	AppRole  *SecretStoreVaultAppRoleAuth  `json:"appRole,omitempty"`
}

type SecretStoreVaultSpec struct {
	Address    string               `json:"address"`
	Mount      string               `json:"mount,omitempty"`
	PathPrefix string               `json:"pathPrefix,omitempty"`
	KVVersion  int                  `json:"kvVersion,omitempty"`
	Auth       SecretStoreVaultAuth `json:"auth"`
	TLS        *TLSSpec             `json:"tls,omitempty"`
	Proxy      *HTTPProxySpec       `json:"proxy,omitempty"`
}

type SecretStoreFileEncryption struct {
	KeyRef        *corev1.SecretKeySelector `json:"keyRef,omitempty"`
	PassphraseRef *corev1.SecretKeySelector `json:"passphraseRef,omitempty"`
}

type SecretStoreFileSpec struct {
	Path       string                    `json:"path"`
	Storage    StorageSpec               `json:"storage"`
	Encryption SecretStoreFileEncryption `json:"encryption"`
}

type SecretStoreSpec struct {
	Vault *SecretStoreVaultSpec `json:"vault,omitempty"`
	File  *SecretStoreFileSpec  `json:"file,omitempty"`
}

type SecretStoreStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	ResolvedPath       string             `json:"resolvedPath,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sst
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
type SecretStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretStoreSpec   `json:"spec,omitempty"`
	Status SecretStoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type SecretStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretStore `json:"items"`
}

func (s *SecretStore) ValidateSpec() error {
	if s == nil {
		return fmt.Errorf("secret store is required")
	}
	hasVault := s.Spec.Vault != nil
	hasFile := s.Spec.File != nil
	if hasVault == hasFile {
		return fmt.Errorf("spec must define exactly one of file or vault")
	}

	if hasVault {
		vault := s.Spec.Vault
		if err := validateHTTPURL(vault.Address, "spec.vault.address"); err != nil {
			return err
		}
		hasToken := vault.Auth.Token != nil
		hasUserPass := vault.Auth.Userpass != nil
		hasAppRole := vault.Auth.AppRole != nil
		if countTrue(hasToken, hasUserPass, hasAppRole) != 1 {
			return fmt.Errorf("spec.vault.auth must define exactly one of token, userpass, or appRole")
		}
		if hasToken {
			if err := validateSecretRef(vault.Auth.Token.SecretRef, "spec.vault.auth.token.secretRef"); err != nil {
				return err
			}
		}
		if hasUserPass {
			if err := validateSecretRef(vault.Auth.Userpass.UsernameRef, "spec.vault.auth.userpass.usernameRef"); err != nil {
				return err
			}
			if err := validateSecretRef(vault.Auth.Userpass.PasswordRef, "spec.vault.auth.userpass.passwordRef"); err != nil {
				return err
			}
		}
		if hasAppRole {
			if err := validateSecretRef(vault.Auth.AppRole.RoleIDRef, "spec.vault.auth.appRole.roleIDRef"); err != nil {
				return err
			}
			if err := validateSecretRef(vault.Auth.AppRole.SecretIDRef, "spec.vault.auth.appRole.secretIDRef"); err != nil {
				return err
			}
		}
		if vault.Proxy != nil {
			hasHTTP := strings.TrimSpace(vault.Proxy.HTTPURL) != ""
			hasHTTPS := strings.TrimSpace(vault.Proxy.HTTPSURL) != ""
			if !hasHTTP && !hasHTTPS {
				return fmt.Errorf("spec.vault.proxy must define at least one of httpURL or httpsURL")
			}
			if vault.Proxy.Auth != nil {
				if err := validateSecretRef(vault.Proxy.Auth.UsernameRef, "spec.vault.proxy.auth.usernameRef"); err != nil {
					return err
				}
				if err := validateSecretRef(vault.Proxy.Auth.PasswordRef, "spec.vault.proxy.auth.passwordRef"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if strings.TrimSpace(s.Spec.File.Path) == "" {
		return fmt.Errorf("spec.file.path is required")
	}
	if err := s.Spec.File.Storage.validate("spec.file.storage"); err != nil {
		return err
	}
	hasKey := s.Spec.File.Encryption.KeyRef != nil
	hasPassphrase := s.Spec.File.Encryption.PassphraseRef != nil
	if hasKey == hasPassphrase {
		return fmt.Errorf("spec.file.encryption must define exactly one of keyRef or passphraseRef")
	}
	if hasKey {
		if err := validateSecretRef(s.Spec.File.Encryption.KeyRef, "spec.file.encryption.keyRef"); err != nil {
			return err
		}
	}
	if hasPassphrase {
		if err := validateSecretRef(s.Spec.File.Encryption.PassphraseRef, "spec.file.encryption.passphraseRef"); err != nil {
			return err
		}
	}
	return nil
}
