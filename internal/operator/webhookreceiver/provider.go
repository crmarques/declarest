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

package webhookreceiver

import (
	"net/http"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

// WebhookEvent contains the parsed metadata from a provider webhook delivery.
type WebhookEvent struct {
	Provider   declarestv1alpha1.RepositoryWebhookProvider
	EventType  string
	DeliveryID string
	Ref        string
	IsPush     bool
	IsPing     bool
}

// Provider defines the interface for webhook provider adapters.
type Provider interface {
	// Name returns the provider identifier.
	Name() declarestv1alpha1.RepositoryWebhookProvider

	// VerifySignature verifies the webhook payload signature or token.
	VerifySignature(req *http.Request, body []byte, secret string) error

	// ParseEvent extracts event metadata from the request and body.
	ParseEvent(req *http.Request, body []byte) (WebhookEvent, error)
}

// ProviderRegistry maps provider names to their adapter implementations.
type ProviderRegistry map[declarestv1alpha1.RepositoryWebhookProvider]Provider

// NewProviderRegistry returns a registry with all built-in providers.
func NewProviderRegistry() ProviderRegistry {
	return ProviderRegistry{
		declarestv1alpha1.RepositoryWebhookProviderGitHub:      &GitHubProvider{},
		declarestv1alpha1.RepositoryWebhookProviderGitLab:      &GitLabProvider{},
		declarestv1alpha1.RepositoryWebhookProviderGitea:       &GiteaProvider{},
		declarestv1alpha1.RepositoryWebhookProviderGenericHMAC: &GenericHMACProvider{},
	}
}
