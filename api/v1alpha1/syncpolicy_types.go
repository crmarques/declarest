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
	"fmt"
	"strings"
	"time"

	"github.com/crmarques/declarest/internal/cronexpr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncPolicySource struct {
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

type SyncPolicySyncOptions struct {
	Prune bool `json:"prune,omitempty"`
	Force bool `json:"force,omitempty"`
}

type SyncPolicySpec struct {
	ResourceRepositoryRef NamespacedObjectReference `json:"resourceRepositoryRef"`
	ManagedServerRef      NamespacedObjectReference `json:"managedServerRef"`
	SecretStoreRef        NamespacedObjectReference `json:"secretStoreRef"`
	Source                SyncPolicySource          `json:"source"`
	Sync                  SyncPolicySyncOptions     `json:"sync,omitempty"`
	SyncInterval          *metav1.Duration          `json:"syncInterval,omitempty"`
	FullResyncCron        string                    `json:"fullResyncCron,omitempty"`
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
	LastFullResyncTime            *metav1.Time            `json:"lastFullResyncTime,omitempty"`
	LastAttemptedRepoRevision     string                  `json:"lastAttemptedRepoRevision,omitempty"`
	LastAppliedRepoRevision       string                  `json:"lastAppliedRepoRevision,omitempty"`
	LastSyncMode                  string                  `json:"lastSyncMode,omitempty"`
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
	if strings.TrimSpace(s.Spec.FullResyncCron) != "" {
		if _, err := cronexpr.Parse(s.Spec.FullResyncCron); err != nil {
			return fmt.Errorf("spec.fullResyncCron is invalid: %w", err)
		}
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
