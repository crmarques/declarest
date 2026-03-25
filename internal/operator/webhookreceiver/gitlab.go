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
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

type GitLabProvider struct{}

func (p *GitLabProvider) Name() declarestv1alpha1.RepositoryWebhookProvider {
	return declarestv1alpha1.RepositoryWebhookProviderGitLab
}

func (p *GitLabProvider) VerifySignature(req *http.Request, _ []byte, secret string) error {
	token := strings.TrimSpace(req.Header.Get("X-Gitlab-Token"))
	if token == "" {
		return fmt.Errorf("missing X-Gitlab-Token header")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return fmt.Errorf("invalid token")
	}
	return nil
}

func (p *GitLabProvider) ParseEvent(req *http.Request, body []byte) (WebhookEvent, error) {
	eventType := strings.TrimSpace(req.Header.Get("X-Gitlab-Event"))
	deliveryID := strings.TrimSpace(req.Header.Get("X-Gitlab-Event-UUID"))

	evt := WebhookEvent{
		Provider:   declarestv1alpha1.RepositoryWebhookProviderGitLab,
		EventType:  eventType,
		DeliveryID: deliveryID,
		IsPush:     strings.EqualFold(eventType, "Push Hook"),
		IsPing:     false, // GitLab has no ping event
	}

	if evt.IsPush {
		var payload struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return evt, fmt.Errorf("invalid payload: %w", err)
		}
		evt.Ref = strings.TrimSpace(payload.Ref)
	}

	return evt, nil
}
