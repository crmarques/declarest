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

package proxy

import (
	"context"
	"maps"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/promptauth"
)

func TestResolveMergesEnvironmentAndConfiguredFields(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy-http.example.com:3128")
	t.Setenv("HTTPS_PROXY", "https://proxy-https.example.com:8443")
	t.Setenv("NO_PROXY", "svc.cluster.local")

	cfg, disabled, err := Resolve("managedServer.http.proxy", &config.HTTPProxy{
		NoProxy: "localhost,127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if disabled {
		t.Fatal("expected proxy resolution to stay enabled")
	}
	if cfg.HTTP == nil || cfg.HTTP.String() != "http://proxy-http.example.com:3128" {
		t.Fatalf("expected resolved http proxy from environment, got %#v", cfg.HTTP)
	}
	if cfg.HTTPS == nil || cfg.HTTPS.String() != "https://proxy-https.example.com:8443" {
		t.Fatalf("expected resolved https proxy from environment, got %#v", cfg.HTTPS)
	}
	if cfg.NoProxy != "localhost,127.0.0.1" {
		t.Fatalf("expected configured noProxy override, got %q", cfg.NoProxy)
	}
}

func TestResolveExplicitDisableSuppressesEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy-http.example.com:3128")
	t.Setenv("HTTPS_PROXY", "https://proxy-https.example.com:8443")
	t.Setenv("NO_PROXY", "svc.cluster.local")

	cfg, disabled, err := Resolve("managedServer.http.proxy", &config.HTTPProxy{})
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if !disabled {
		t.Fatal("expected explicit empty proxy block to disable environment proxy")
	}
	if cfg.HasProxy() || cfg.NoProxy != "" {
		t.Fatalf("expected disabled proxy config to be empty, got %#v", cfg)
	}
}

func TestResolveConfiguredAuthOverridesEnvironmentCredentials(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://env-user:env-pass@proxy.example.com:3128")

	cfg, disabled, err := Resolve("managedServer.http.proxy", &config.HTTPProxy{
		Auth: &config.ProxyAuth{
			Basic: &config.BasicAuth{
				Username: "ctx-user",
				Password: "ctx-pass",
			},
		},
	})
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if disabled {
		t.Fatal("expected proxy resolution to stay enabled")
	}
	if cfg.HTTP == nil {
		t.Fatal("expected resolved http proxy")
	}
	if got := cfg.HTTP.String(); got != "http://ctx-user:ctx-pass@proxy.example.com:3128" {
		t.Fatalf("expected configured auth to override env credentials, got %q", got)
	}
}

func TestResolveWithRuntimePromptAuthInjectsPromptedCredentials(t *testing.T) {
	runtime, err := promptauth.New(
		[]promptauth.Target{{Key: promptauth.TargetManagedServerHTTPProxyAuth, Label: "managed-server proxy auth"}},
		promptauth.WithPrompter(&proxyPromptPrompter{
			credentials: promptauth.Credentials{Username: "proxy-user", Password: "proxy-pass"},
		}),
		promptauth.WithSessionStore(&proxyMemorySessionStore{}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	cfg, disabled, err := ResolveWithRuntime("managed-server.http.proxy", &config.HTTPProxy{
		HTTPURL: "http://proxy.example.com:3128",
		Auth: &config.ProxyAuth{
			Prompt: &config.PromptAuth{},
		},
	}, runtime)
	if err != nil {
		t.Fatalf("ResolveWithRuntime() returned error: %v", err)
	}
	if disabled {
		t.Fatal("expected prompt-backed proxy to remain enabled")
	}

	env, err := cfg.Env(context.Background())
	if err != nil {
		t.Fatalf("Env() returned error: %v", err)
	}
	if got := env["HTTP_PROXY"]; got != "http://proxy-user:proxy-pass@proxy.example.com:3128" {
		t.Fatalf("expected prompted proxy URL, got %q", got)
	}
}

type proxyPromptPrompter struct {
	credentials promptauth.Credentials
}

func (p *proxyPromptPrompter) PromptCredentials(context.Context, promptauth.Target, bool, bool) (promptauth.Credentials, error) {
	return p.credentials, nil
}

func (p *proxyPromptPrompter) ConfirmReuse(context.Context, promptauth.Target, []promptauth.Target) (bool, error) {
	return false, nil
}

type proxyMemorySessionStore struct {
	values map[string]string
}

func (s *proxyMemorySessionStore) Load() (map[string]string, error) {
	return maps.Clone(s.values), nil
}

func (s *proxyMemorySessionStore) Save(values map[string]string) error {
	s.values = maps.Clone(values)
	return nil
}
