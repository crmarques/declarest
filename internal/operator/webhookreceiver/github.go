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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

type GitHubProvider struct{}

func (p *GitHubProvider) Name() declarestv1alpha1.RepositoryWebhookProvider {
	return declarestv1alpha1.RepositoryWebhookProviderGitHub
}

func (p *GitHubProvider) VerifySignature(req *http.Request, body []byte, secret string) error {
	signature := strings.TrimSpace(req.Header.Get("X-Hub-Signature-256"))
	if signature == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("invalid signature format: missing sha256= prefix")
	}
	sigHex := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)

	provided, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	if subtle.ConstantTimeCompare(provided, expected) != 1 {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func (p *GitHubProvider) ParseEvent(req *http.Request, body []byte) (WebhookEvent, error) {
	eventType := strings.TrimSpace(req.Header.Get("X-GitHub-Event"))
	deliveryID := strings.TrimSpace(req.Header.Get("X-GitHub-Delivery"))

	evt := WebhookEvent{
		Provider:   declarestv1alpha1.RepositoryWebhookProviderGitHub,
		EventType:  eventType,
		DeliveryID: deliveryID,
		IsPush:     strings.EqualFold(eventType, "push"),
		IsPing:     strings.EqualFold(eventType, "ping"),
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
