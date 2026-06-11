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
	"path"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RepositoryWebhookProvider string

const (
	RepositoryWebhookProviderGitHub      RepositoryWebhookProvider = "github"
	RepositoryWebhookProviderGitLab      RepositoryWebhookProvider = "gitlab"
	RepositoryWebhookProviderGitea       RepositoryWebhookProvider = "gitea"
	RepositoryWebhookProviderGenericHMAC RepositoryWebhookProvider = "generic-hmac"
)

type RepositoryWebhookBranchFilter struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// +kubebuilder:validation:Enum=push;ping
type RepositoryWebhookEvent string

const (
	RepositoryWebhookEventPush RepositoryWebhookEvent = "push"
	RepositoryWebhookEventPing RepositoryWebhookEvent = "ping"
)

type RepositoryWebhookSpec struct {
	RepositoryRef NamespacedObjectReference `json:"repositoryRef"`
	// +kubebuilder:validation:Enum=github;gitlab;gitea;generic-hmac
	Provider  RepositoryWebhookProvider  `json:"provider"`
	SecretRef RepositoryWebhookSecretRef `json:"secretRef"`
	// +kubebuilder:validation:MinItems=1
	Events       []RepositoryWebhookEvent       `json:"events,omitempty"`
	BranchFilter *RepositoryWebhookBranchFilter `json:"branchFilter,omitempty"`
	Suspend      bool                           `json:"suspend,omitempty"`
}

type RepositoryWebhookSecretRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

type RepositoryWebhookStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	WebhookPath        string             `json:"webhookPath,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastEventTime      *metav1.Time       `json:"lastEventTime,omitempty"`
	LastDeliveryID     string             `json:"lastDeliveryID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=rwh,categories=declarest
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repositoryRef.name"
// +kubebuilder:printcolumn:name="Path",type="string",JSONPath=".status.webhookPath"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type RepositoryWebhook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositoryWebhookSpec   `json:"spec,omitempty"`
	Status RepositoryWebhookStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type RepositoryWebhookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RepositoryWebhook `json:"items"`
}

func (r *RepositoryWebhook) Default() {
	if r == nil {
		return
	}
	if len(r.Spec.Events) == 0 {
		r.Spec.Events = []RepositoryWebhookEvent{RepositoryWebhookEventPush}
	}
}

func (r *RepositoryWebhook) ValidateSpec() error {
	if r == nil {
		return fmt.Errorf("repository webhook is required")
	}
	if strings.TrimSpace(r.Spec.RepositoryRef.Name) == "" {
		return fmt.Errorf("spec.repositoryRef.name is required")
	}
	switch r.Spec.Provider {
	case RepositoryWebhookProviderGitHub, RepositoryWebhookProviderGitLab, RepositoryWebhookProviderGitea, RepositoryWebhookProviderGenericHMAC:
	default:
		return fmt.Errorf("spec.provider must be github, gitlab, gitea, or generic-hmac")
	}
	if strings.TrimSpace(r.Spec.SecretRef.Name) == "" {
		return fmt.Errorf("spec.secretRef.name is required")
	}
	if strings.TrimSpace(r.Spec.SecretRef.Key) == "" {
		return fmt.Errorf("spec.secretRef.key is required")
	}
	seenEvents := make(map[RepositoryWebhookEvent]struct{}, len(r.Spec.Events))
	for idx, event := range r.Spec.Events {
		switch event {
		case RepositoryWebhookEventPush, RepositoryWebhookEventPing:
		default:
			return fmt.Errorf("spec.events[%d] must be push or ping", idx)
		}
		if _, exists := seenEvents[event]; exists {
			return fmt.Errorf("spec.events[%d] %q is duplicated", idx, event)
		}
		seenEvents[event] = struct{}{}
	}
	if r.Spec.BranchFilter != nil {
		for idx, pattern := range r.Spec.BranchFilter.Include {
			if err := validateBranchPattern(pattern); err != nil {
				return fmt.Errorf("spec.branchFilter.include[%d]: %w", idx, err)
			}
		}
		for idx, pattern := range r.Spec.BranchFilter.Exclude {
			if err := validateBranchPattern(pattern); err != nil {
				return fmt.Errorf("spec.branchFilter.exclude[%d]: %w", idx, err)
			}
		}
	}
	return nil
}

func validateBranchPattern(pattern string) error {
	value := strings.TrimSpace(pattern)
	if value == "" {
		return fmt.Errorf("pattern is required")
	}
	if _, err := path.Match(value, "main"); err != nil {
		return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
	}
	return nil
}
