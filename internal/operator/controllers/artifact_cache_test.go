package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/crmarques/declarest/config"
)

func TestArtifactPathExtension(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "tar gz extension",
			path: "/releases/keycloak-bundle-0.0.1.tar.gz",
			want: ".tar.gz",
		},
		{
			name: "tgz extension",
			path: "/releases/keycloak-bundle-0.0.1.tgz",
			want: ".tgz",
		},
		{
			name: "json extension",
			path: "/openapi.json",
			want: ".json",
		},
		{
			name: "no extension",
			path: "/artifact",
			want: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := artifactPathExtension(tc.path)
			if got != tc.want {
				t.Fatalf("artifactPathExtension(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestDownloadArtifactPreservesTarGzExtension(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bundle-bytes"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	targetURL := server.URL + "/keycloak-bundle-0.0.1.tar.gz"

	path, err := downloadArtifact(context.Background(), targetURL, tempDir, nil)
	if err != nil {
		t.Fatalf("downloadArtifact returned error: %v", err)
	}
	if !strings.HasSuffix(path, ".tar.gz") {
		t.Fatalf("expected cached path to end with .tar.gz, got %q", path)
	}
	if filepath.Dir(path) != tempDir {
		t.Fatalf("expected cached file in %q, got %q", tempDir, path)
	}
}

func TestDownloadArtifactUsesMergedProxyConfiguration(t *testing.T) {
	var proxyRequests int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxyRequests, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bundle-bytes"))
	}))
	defer proxy.Close()

	t.Setenv("HTTP_PROXY", proxy.URL)

	tempDir := t.TempDir()
	path, err := downloadArtifact(
		context.Background(),
		"http://artifact.example.com/keycloak-bundle-0.0.1.tar.gz",
		tempDir,
		&config.HTTPProxy{NoProxy: "localhost"},
	)
	if err != nil {
		t.Fatalf("downloadArtifact returned error: %v", err)
	}
	if got := atomic.LoadInt32(&proxyRequests); got != 1 {
		t.Fatalf("expected artifact download to use proxy once, got %d", got)
	}
	if !strings.HasSuffix(path, ".tar.gz") {
		t.Fatalf("expected cached path to end with .tar.gz, got %q", path)
	}
}

func TestDownloadArtifactExplicitProxyDisableSuppressesEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9")

	_, err := downloadArtifact(
		context.Background(),
		"http://artifact.example.com/keycloak-bundle-0.0.1.tar.gz",
		t.TempDir(),
		&config.HTTPProxy{},
	)
	if err == nil {
		t.Fatal("expected proxy disable test request to fail without environment proxy")
	}
	if strings.Contains(err.Error(), "127.0.0.1:9") {
		t.Fatalf("expected explicit disable to suppress environment proxy, got %v", err)
	}
}
