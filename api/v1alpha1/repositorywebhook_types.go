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

type RepositoryWebhookEvent string

const (
	RepositoryWebhookEventPush RepositoryWebhookEvent = "push"
	RepositoryWebhookEventPing RepositoryWebhookEvent = "ping"
)

type RepositoryWebhookSpec struct {
	RepositoryRef NamespacedObjectReference `json:"repositoryRef"`
	// +kubebuilder:validation:Enum=github;gitlab;gitea;generic-hmac
	Provider     RepositoryWebhookProvider      `json:"provider"`
	SecretRef    NamespacedObjectReference       `json:"secretRef"`
	Events       []RepositoryWebhookEvent        `json:"events,omitempty"`
	BranchFilter *RepositoryWebhookBranchFilter  `json:"branchFilter,omitempty"`
	Suspend      bool                            `json:"suspend,omitempty"`
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
// +kubebuilder:resource:scope=Namespaced,shortName=rwh
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider"
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.repositoryRef.name"
// +kubebuilder:printcolumn:name="Path",type="string",JSONPath=".status.webhookPath"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
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
