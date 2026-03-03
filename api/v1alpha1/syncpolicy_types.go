package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncPolicySource struct {
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

type SyncPolicySyncOptions struct {
	Prune bool `json:"prune,omitempty"`
}

type SyncPolicySpec struct {
	ResourceRepositoryRef NamespacedObjectReference `json:"resourceRepositoryRef"`
	ManagedServerRef      NamespacedObjectReference `json:"managedServerRef"`
	SecretStoreRef        NamespacedObjectReference `json:"secretStoreRef"`
	Source                SyncPolicySource          `json:"source"`
	Sync                  SyncPolicySyncOptions     `json:"sync,omitempty"`
	SyncInterval          *metav1.Duration          `json:"syncInterval,omitempty"`
	Suspend               bool                      `json:"suspend,omitempty"`
}

type SyncPolicyResourceStats struct {
	Targeted int32 `json:"targeted,omitempty"`
	Applied  int32 `json:"applied,omitempty"`
	Pruned   int32 `json:"pruned,omitempty"`
	Failed   int32 `json:"failed,omitempty"`
}

type SyncPolicyStatus struct {
	ObservedGeneration            int64                   `json:"observedGeneration,omitempty"`
	LastAttemptTime               *metav1.Time            `json:"lastAttemptTime,omitempty"`
	LastSuccessfulSyncTime        *metav1.Time            `json:"lastSuccessfulSyncTime,omitempty"`
	LastAttemptedRepoRevision     string                  `json:"lastAttemptedRepoRevision,omitempty"`
	LastAppliedRepoRevision       string                  `json:"lastAppliedRepoRevision,omitempty"`
	LastSecretResourceVersionHash string                  `json:"lastSecretResourceVersionHash,omitempty"`
	ResourceStats                 SyncPolicyResourceStats `json:"resourceStats,omitempty"`
	Conditions                    []metav1.Condition      `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sp
// +kubebuilder:printcolumn:name="Repo",type="string",JSONPath=".spec.resourceRepositoryRef.name"
// +kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.managedServerRef.name"
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.lastAppliedRepoRevision"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
type SyncPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncPolicySpec   `json:"spec,omitempty"`
	Status SyncPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type SyncPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncPolicy `json:"items"`
}

const defaultSyncInterval = 5 * time.Minute

func (s *SyncPolicy) Default() {
	if s.Spec.Source.Recursive == nil {
		value := true
		s.Spec.Source.Recursive = &value
	}
	if s.Spec.SyncInterval == nil {
		s.Spec.SyncInterval = &metav1.Duration{Duration: defaultSyncInterval}
	}
}

func (s *SyncPolicy) ValidateSpec() error {
	if s == nil {
		return fmt.Errorf("sync policy is required")
	}
	if strings.TrimSpace(s.Spec.ResourceRepositoryRef.Name) == "" {
		return fmt.Errorf("spec.resourceRepositoryRef.name is required")
	}
	if strings.TrimSpace(s.Spec.ManagedServerRef.Name) == "" {
		return fmt.Errorf("spec.managedServerRef.name is required")
	}
	if strings.TrimSpace(s.Spec.SecretStoreRef.Name) == "" {
		return fmt.Errorf("spec.secretStoreRef.name is required")
	}
	normalizedPath, err := normalizePath(s.Spec.Source.Path)
	if err != nil {
		return fmt.Errorf("spec.source.path is invalid: %w", err)
	}
	s.Spec.Source.Path = normalizedPath
	if s.Spec.Source.Recursive == nil {
		value := true
		s.Spec.Source.Recursive = &value
	}
	return nil
}
