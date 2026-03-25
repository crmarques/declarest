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
	"encoding/hex"
	"net/http"
	"testing"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

func TestGitHubProviderVerifySignature(t *testing.T) {
	p := &GitHubProvider{}
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "test-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Hub-Signature-256", sig)

	if err := p.VerifySignature(req, body, secret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	req.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
	if err := p.VerifySignature(req, body, secret); err == nil {
		t.Fatal("invalid signature accepted")
	}

	req.Header.Del("X-Hub-Signature-256")
	if err := p.VerifySignature(req, body, secret); err == nil {
		t.Fatal("missing signature accepted")
	}
}

func TestGitHubProviderParseEvent(t *testing.T) {
	p := &GitHubProvider{}

	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "abc-123")

	body := []byte(`{"ref":"refs/heads/main"}`)
	evt, err := p.ParseEvent(req, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evt.IsPush {
		t.Error("expected IsPush=true")
	}
	if evt.IsPing {
		t.Error("expected IsPing=false")
	}
	if evt.Ref != "refs/heads/main" {
		t.Errorf("expected ref=refs/heads/main, got %q", evt.Ref)
	}
	if evt.DeliveryID != "abc-123" {
		t.Errorf("expected deliveryID=abc-123, got %q", evt.DeliveryID)
	}

	req.Header.Set("X-GitHub-Event", "ping")
	evt, err = p.ParseEvent(req, []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.IsPush {
		t.Error("expected IsPush=false for ping")
	}
	if !evt.IsPing {
		t.Error("expected IsPing=true for ping")
	}
}

func TestGitLabProviderVerifySignature(t *testing.T) {
	p := &GitLabProvider{}
	secret := "my-gitlab-token"

	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitlab-Token", secret)

	if err := p.VerifySignature(req, nil, secret); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	req.Header.Set("X-Gitlab-Token", "wrong-token")
	if err := p.VerifySignature(req, nil, secret); err == nil {
		t.Fatal("invalid token accepted")
	}
}

func TestGiteaProviderVerifySignature(t *testing.T) {
	p := &GiteaProvider{}
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "gitea-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Gitea-Signature", sig)

	if err := p.VerifySignature(req, body, secret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	req.Header.Set("X-Gitea-Signature", "0000000000000000000000000000000000000000000000000000000000000000")
	if err := p.VerifySignature(req, body, secret); err == nil {
		t.Fatal("invalid signature accepted")
	}
}

func TestGenericHMACProviderVerifySignature(t *testing.T) {
	p := &GenericHMACProvider{}
	body := []byte(`{"ref":"refs/heads/main"}`)
	secret := "generic-secret"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Signature", sig)

	if err := p.VerifySignature(req, body, secret); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestNewProviderRegistry(t *testing.T) {
	reg := NewProviderRegistry()
	expected := []declarestv1alpha1.RepositoryWebhookProvider{
		declarestv1alpha1.RepositoryWebhookProviderGitHub,
		declarestv1alpha1.RepositoryWebhookProviderGitLab,
		declarestv1alpha1.RepositoryWebhookProviderGitea,
		declarestv1alpha1.RepositoryWebhookProviderGenericHMAC,
	}
	for _, p := range expected {
		if _, ok := reg[p]; !ok {
			t.Errorf("provider %q not in registry", p)
		}
	}
}
