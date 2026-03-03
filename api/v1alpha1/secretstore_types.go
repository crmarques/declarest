package v1alpha1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretStoreProvider string

const (
	SecretStoreProviderVault SecretStoreProvider = "vault"
	SecretStoreProviderFile  SecretStoreProvider = "file"
)

type SecretStoreVaultAuth struct {
	TokenRef           *corev1.SecretKeySelector `json:"tokenRef,omitempty"`
	UsernameRef        *corev1.SecretKeySelector `json:"usernameRef,omitempty"`
	PasswordRef        *corev1.SecretKeySelector `json:"passwordRef,omitempty"`
	UserpassMount      string                    `json:"userpassMount,omitempty"`
	AppRoleRoleIDRef   *corev1.SecretKeySelector `json:"appRoleRoleIDRef,omitempty"`
	AppRoleSecretIDRef *corev1.SecretKeySelector `json:"appRoleSecretIDRef,omitempty"`
	AppRoleMount       string                    `json:"appRoleMount,omitempty"`
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
	Provider SecretStoreProvider   `json:"provider"`
	Vault    *SecretStoreVaultSpec `json:"vault,omitempty"`
	File     *SecretStoreFileSpec  `json:"file,omitempty"`
}

type SecretStoreStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	ResolvedPath       string             `json:"resolvedPath,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ss
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
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
	if s.Spec.Provider != SecretStoreProviderVault && s.Spec.Provider != SecretStoreProviderFile {
		return fmt.Errorf("spec.provider must be one of: vault, file")
	}
	if s.Spec.Provider == SecretStoreProviderVault {
		if s.Spec.Vault == nil {
			return fmt.Errorf("spec.vault is required when provider=vault")
		}
		if s.Spec.File != nil {
			return fmt.Errorf("spec.file must not be set when provider=vault")
		}
		if err := validateHTTPURL(s.Spec.Vault.Address, "spec.vault.address"); err != nil {
			return err
		}
		hasToken := s.Spec.Vault.Auth.TokenRef != nil
		hasUserPass := s.Spec.Vault.Auth.UsernameRef != nil || s.Spec.Vault.Auth.PasswordRef != nil
		hasAppRole := s.Spec.Vault.Auth.AppRoleRoleIDRef != nil || s.Spec.Vault.Auth.AppRoleSecretIDRef != nil
		if countTrue(hasToken, hasUserPass, hasAppRole) != 1 {
			return fmt.Errorf("spec.vault.auth must define exactly one of tokenRef, username/password refs, or appRole refs")
		}
		if hasToken {
			if err := validateSecretRef(s.Spec.Vault.Auth.TokenRef, "spec.vault.auth.tokenRef"); err != nil {
				return err
			}
		}
		if hasUserPass {
			if err := validateSecretRef(s.Spec.Vault.Auth.UsernameRef, "spec.vault.auth.usernameRef"); err != nil {
				return err
			}
			if err := validateSecretRef(s.Spec.Vault.Auth.PasswordRef, "spec.vault.auth.passwordRef"); err != nil {
				return err
			}
		}
		if hasAppRole {
			if err := validateSecretRef(s.Spec.Vault.Auth.AppRoleRoleIDRef, "spec.vault.auth.appRoleRoleIDRef"); err != nil {
				return err
			}
			if err := validateSecretRef(s.Spec.Vault.Auth.AppRoleSecretIDRef, "spec.vault.auth.appRoleSecretIDRef"); err != nil {
				return err
			}
		}
		if s.Spec.Vault.Proxy != nil {
			hasHTTP := strings.TrimSpace(s.Spec.Vault.Proxy.HTTPURL) != ""
			hasHTTPS := strings.TrimSpace(s.Spec.Vault.Proxy.HTTPSURL) != ""
			if !hasHTTP && !hasHTTPS {
				return fmt.Errorf("spec.vault.proxy must define at least one of httpURL or httpsURL")
			}
			if s.Spec.Vault.Proxy.Auth != nil {
				if err := validateSecretRef(s.Spec.Vault.Proxy.Auth.UsernameRef, "spec.vault.proxy.auth.usernameRef"); err != nil {
					return err
				}
				if err := validateSecretRef(s.Spec.Vault.Proxy.Auth.PasswordRef, "spec.vault.proxy.auth.passwordRef"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if s.Spec.File == nil {
		return fmt.Errorf("spec.file is required when provider=file")
	}
	if s.Spec.Vault != nil {
		return fmt.Errorf("spec.vault must not be set when provider=file")
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
