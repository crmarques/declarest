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

import "testing"

func TestRepositoryWebhookDefaultEventsToPush(t *testing.T) {
	t.Parallel()

	webhook := validRepositoryWebhook()
	webhook.Spec.Events = nil

	webhook.Default()

	if len(webhook.Spec.Events) != 1 || webhook.Spec.Events[0] != RepositoryWebhookEventPush {
		t.Fatalf("expected default push event, got %#v", webhook.Spec.Events)
	}
	if err := webhook.ValidateSpec(); err != nil {
		t.Fatalf("ValidateSpec() unexpected error after defaulting: %v", err)
	}
}

func TestRepositoryWebhookRequiresSecretKey(t *testing.T) {
	t.Parallel()

	webhook := validRepositoryWebhook()
	webhook.Spec.SecretRef.Key = ""

	if err := webhook.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected missing secret key error, got nil")
	}
}

func TestRepositoryWebhookRejectsDuplicateEvents(t *testing.T) {
	t.Parallel()

	webhook := validRepositoryWebhook()
	webhook.Spec.Events = []RepositoryWebhookEvent{RepositoryWebhookEventPush, RepositoryWebhookEventPush}

	if err := webhook.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected duplicate event error, got nil")
	}
}

func TestRepositoryWebhookRejectsInvalidBranchGlob(t *testing.T) {
	t.Parallel()

	webhook := validRepositoryWebhook()
	webhook.Spec.BranchFilter = &RepositoryWebhookBranchFilter{
		Include: []string{"release/["},
	}

	if err := webhook.ValidateSpec(); err == nil {
		t.Fatal("ValidateSpec() expected invalid branch glob error, got nil")
	}
}

func validRepositoryWebhook() *RepositoryWebhook {
	return &RepositoryWebhook{
		Spec: RepositoryWebhookSpec{
			RepositoryRef: NamespacedObjectReference{Name: "repo"},
			Provider:      RepositoryWebhookProviderGitHub,
			SecretRef:     RepositoryWebhookSecretRef{Name: "webhook-secret", Key: "token"},
			Events:        []RepositoryWebhookEvent{RepositoryWebhookEventPush},
		},
	}
}
