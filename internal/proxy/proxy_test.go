package proxy

import (
	"testing"

	"github.com/crmarques/declarest/config"
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
			Username: "ctx-user",
			Password: "ctx-pass",
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
