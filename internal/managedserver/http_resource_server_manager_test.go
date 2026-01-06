package managedserver

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPResourceServerManagerDefaultsToTLSVerify(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	tr, ok := manager.client.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("expected http.Transport, got %#v", manager.client.Transport)
	}
	if tr.TLSClientConfig == nil {
		t.Fatalf("expected TLS config to be set")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected TLS verification to be enabled by default")
	}
}

func TestHTTPResourceServerManagerHonorsInsecureSkipVerify(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		TLS: &HTTPResourceServerTLSConfig{
			InsecureSkipVerify: true,
		},
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	tr, ok := manager.client.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatalf("expected http.Transport, got %#v", manager.client.Transport)
	}
	if tr.TLSClientConfig == nil {
		t.Fatalf("expected TLS config to be set")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected TLS insecure skip verify to be enabled")
	}
}

func TestHTTPResourceServerManagerAppliesBearerToken(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		Auth: &HTTPResourceServerAuthConfig{
			BearerToken: &HTTPResourceServerBearerTokenConfig{
				Token: "token-123",
			},
		},
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := manager.applyAuth(context.Background(), req); err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
		t.Fatalf("expected bearer token, got %q", got)
	}
}

func TestHTTPResourceServerManagerAppliesCustomHeader(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		Auth: &HTTPResourceServerAuthConfig{
			CustomHeader: &HTTPResourceServerCustomHeaderConfig{
				Header: "X-Test-Auth",
				Token:  "token-123",
			},
		},
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := manager.applyAuth(context.Background(), req); err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	if got := req.Header.Get("X-Test-Auth"); got != "token-123" {
		t.Fatalf("expected custom header token, got %q", got)
	}
}

func TestHTTPResourceServerManagerAppliesBasicAuth(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		Auth: &HTTPResourceServerAuthConfig{
			BasicAuth: &HTTPResourceServerBasicAuthConfig{
				Username: "user",
				Password: "pass",
			},
		},
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := manager.applyAuth(context.Background(), req); err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "user" || pass != "pass" {
		t.Fatalf("expected basic auth user/pass, got %q/%q (ok=%v)", user, pass, ok)
	}
}

func TestHTTPResourceServerManagerOAuthTokenCached(t *testing.T) {
	var calls int32

	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		Auth: &HTTPResourceServerAuthConfig{
			OAuth2: &HTTPResourceServerOAuth2Config{
				TokenURL:     "https://example.com/token",
				GrantType:    "client_credentials",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
	})
	manager.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST token request, got %s", req.Method)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if values.Get("grant_type") != "client_credentials" {
				t.Fatalf("unexpected grant_type %q", values.Get("grant_type"))
			}
			if values.Get("client_id") != "client-id" || values.Get("client_secret") != "client-secret" {
				t.Fatalf("unexpected client credentials %q/%q", values.Get("client_id"), values.Get("client_secret"))
			}

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"token-1","expires_in":3600}`)),
				Request:    req,
			}
			return resp, nil
		}),
	}

	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := manager.applyAuth(context.Background(), req1); err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	if got := req1.Header.Get("Authorization"); got != "Bearer token-1" {
		t.Fatalf("expected bearer token, got %q", got)
	}

	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := manager.applyAuth(context.Background(), req2); err != nil {
		t.Fatalf("applyAuth: %v", err)
	}
	if got := req2.Header.Get("Authorization"); got != "Bearer token-1" {
		t.Fatalf("expected bearer token, got %q", got)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected token endpoint called once, got %d", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
