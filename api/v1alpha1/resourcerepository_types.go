package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceRepositoryType string

const (
	ResourceRepositoryTypeGit ResourceRepositoryType = "git"
)

type ResourceRepositoryAuth struct {
	TokenRef     *corev1.SecretKeySelector `json:"tokenRef,omitempty"`
	SSHSecretRef *GitSSHSecretRef          `json:"sshSecretRef,omitempty"`
}

type GitSSHSecretRef struct {
	PrivateKeyRef         *corev1.SecretKeySelector `json:"privateKeyRef,omitempty"`
	KnownHostsRef         *corev1.SecretKeySelector `json:"knownHostsRef,omitempty"`
	PassphraseRef         *corev1.SecretKeySelector `json:"passphraseRef,omitempty"`
	User                  string                    `json:"user,omitempty"`
	InsecureIgnoreHostKey bool                      `json:"insecureIgnoreHostKey,omitempty"`
}

type GitRepositorySpec struct {
	URL     string                    `json:"url"`
	Branch  string                    `json:"branch,omitempty"`
	Auth    ResourceRepositoryAuth    `json:"auth"`
	Webhook *GitRepositoryWebhookSpec `json:"webhook,omitempty"`
}

type GitWebhookProvider string

const (
	GitWebhookProviderGitea  GitWebhookProvider = "gitea"
	GitWebhookProviderGitLab GitWebhookProvider = "gitlab"
)

type GitRepositoryWebhookSpec struct {
	// +kubebuilder:validation:Enum=gitea;gitlab
	Provider  GitWebhookProvider        `json:"provider"`
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

type ResourceRepositorySpec struct {
	Type         ResourceRepositoryType `json:"type"`
	PollInterval metav1.Duration        `json:"pollInterval"`
	Git          *GitRepositorySpec     `json:"git,omitempty"`
	Storage      StorageSpec            `json:"storage"`
}

type ResourceRepositoryStatus struct {
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	LocalPath           string             `json:"localPath,omitempty"`
	LastFetchedRevision string             `json:"lastFetchedRevision,omitempty"`
	LastFetchedTime     *metav1.Time       `json:"lastFetchedTime,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rr
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.lastFetchedRevision"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
type ResourceRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceRepositorySpec   `json:"spec,omitempty"`
	Status ResourceRepositoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ResourceRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceRepository `json:"items"`
}

func (r *ResourceRepository) Default() {
	if r.Spec.Git != nil && strings.TrimSpace(r.Spec.Git.Branch) == "" {
		r.Spec.Git.Branch = "main"
	}
}

func (r *ResourceRepository) ValidateSpec() error {
	if r == nil {
		return fmt.Errorf("resource repository is required")
	}
	if r.Spec.Type != ResourceRepositoryTypeGit {
		return fmt.Errorf("spec.type must be %q", ResourceRepositoryTypeGit)
	}
	if r.Spec.PollInterval.Duration <= 0 {
		return fmt.Errorf("spec.pollInterval must be greater than zero")
	}
	if r.Spec.PollInterval.Duration < 30*time.Second {
		return fmt.Errorf("spec.pollInterval must be at least 30s")
	}
	if r.Spec.Git == nil {
		return fmt.Errorf("spec.git is required")
	}
	if err := validateGitURL(r.Spec.Git.URL, "spec.git.url"); err != nil {
		return err
	}
	if strings.TrimSpace(r.Spec.Git.Branch) == "" {
		return fmt.Errorf("spec.git.branch is required")
	}
	hasToken := r.Spec.Git.Auth.TokenRef != nil
	hasSSH := r.Spec.Git.Auth.SSHSecretRef != nil
	if hasToken == hasSSH {
		return fmt.Errorf("spec.git.auth must define exactly one of tokenRef or sshSecretRef")
	}
	if hasToken {
		if err := validateSecretRef(r.Spec.Git.Auth.TokenRef, "spec.git.auth.tokenRef"); err != nil {
			return err
		}
	}
	if hasSSH {
		if r.Spec.Git.Auth.SSHSecretRef.PrivateKeyRef == nil {
			return fmt.Errorf("spec.git.auth.sshSecretRef.privateKeyRef is required")
		}
		if err := validateSecretRef(r.Spec.Git.Auth.SSHSecretRef.PrivateKeyRef, "spec.git.auth.sshSecretRef.privateKeyRef"); err != nil {
			return err
		}
		if r.Spec.Git.Auth.SSHSecretRef.KnownHostsRef == nil && !r.Spec.Git.Auth.SSHSecretRef.InsecureIgnoreHostKey {
			return fmt.Errorf("spec.git.auth.sshSecretRef.knownHostsRef is required for SSH authentication; set insecureIgnoreHostKey: true to skip host key verification (not recommended)")
		}
		if r.Spec.Git.Auth.SSHSecretRef.KnownHostsRef != nil {
			if err := validateSecretRef(r.Spec.Git.Auth.SSHSecretRef.KnownHostsRef, "spec.git.auth.sshSecretRef.knownHostsRef"); err != nil {
				return err
			}
		}
		if r.Spec.Git.Auth.SSHSecretRef.PassphraseRef != nil {
			if err := validateSecretRef(r.Spec.Git.Auth.SSHSecretRef.PassphraseRef, "spec.git.auth.sshSecretRef.passphraseRef"); err != nil {
				return err
			}
		}
	}
	if r.Spec.Git.Webhook != nil {
		if r.Spec.Git.Webhook.Provider != GitWebhookProviderGitea && r.Spec.Git.Webhook.Provider != GitWebhookProviderGitLab {
			return fmt.Errorf("spec.git.webhook.provider must be one of: gitea, gitlab")
		}
		if err := validateSecretRef(r.Spec.Git.Webhook.SecretRef, "spec.git.webhook.secretRef"); err != nil {
			return err
		}
	}
	if err := r.Spec.Storage.validate("spec.storage"); err != nil {
		return err
	}
	return nil
}
