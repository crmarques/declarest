package managedserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestHTTPResourceServerManagerLoadsCACertFile(t *testing.T) {
	tmp := t.TempDir()
	_, _, certPEM := writeTestCertificateFiles(t, tmp)
	caPath := filepath.Join(tmp, "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o644); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}

	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		TLS: &HTTPResourceServerTLSConfig{
			CACertFile: caPath,
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
	if tr.TLSClientConfig.RootCAs == nil || len(tr.TLSClientConfig.RootCAs.Subjects()) == 0 {
		t.Fatalf("expected RootCAs to contain the configured CA")
	}
	if len(tr.TLSClientConfig.Certificates) != 0 {
		t.Fatalf("expected no client certificates without mTLS configuration")
	}
}

func TestHTTPResourceServerManagerLoadsClientCertificate(t *testing.T) {
	tmp := t.TempDir()
	certPath, keyPath, _ := writeTestCertificateFiles(t, tmp)

	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
		TLS: &HTTPResourceServerTLSConfig{
			ClientCertFile: certPath,
			ClientKeyFile:  keyPath,
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
	if len(tr.TLSClientConfig.Certificates) != 1 {
		t.Fatalf("expected client certificate to be loaded, got %d", len(tr.TLSClientConfig.Certificates))
	}
}

func TestHTTPResourceServerManagerRequiresBothClientCertAndKey(t *testing.T) {
	cases := []struct {
		name string
		cfg  *HTTPResourceServerTLSConfig
	}{
		{
			name: "missing_key",
			cfg: &HTTPResourceServerTLSConfig{
				ClientCertFile: "cert.pem",
			},
		},
		{
			name: "missing_cert",
			cfg: &HTTPResourceServerTLSConfig{
				ClientKeyFile: "key.pem",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
				BaseURL: "https://example.com",
				TLS:     tc.cfg,
			})
			if err := manager.Init(); err == nil {
				t.Fatalf("expected error when %s", tc.name)
			}
		})
	}
}

func TestHTTPResourceServerManagerConnectsWithMutualTLS(t *testing.T) {
	caCert, caKey, caPEM := generateCACertificate(t)
	serverCertPEM, serverKeyPEM := generateSignedCertificate(t, caCert, caKey, x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test-server"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"127.0.0.1", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	})
	clientCertPEM, clientKeyPEM := generateSignedCertificate(t, caCert, caKey, x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	cert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("load server certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
			t.Skipf("skipping mutual TLS e2e in restricted environment: %v", err)
		}
		t.Fatalf("create listener: %v", err)
	}
	tlsListener := tls.NewListener(listener, serverTLS)
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(tlsListener)
	}()
	defer server.Close()
	serverURL := "https://" + listener.Addr().String()

	tmp := t.TempDir()
	caPath := filepath.Join(tmp, "ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o644); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}
	clientCertPath := filepath.Join(tmp, "client.pem")
	if err := os.WriteFile(clientCertPath, clientCertPEM, 0o644); err != nil {
		t.Fatalf("write client cert: %v", err)
	}
	clientKeyPath := filepath.Join(tmp, "client-key.pem")
	if err := os.WriteFile(clientKeyPath, clientKeyPEM, 0o600); err != nil {
		t.Fatalf("write client key: %v", err)
	}

	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: serverURL,
		TLS: &HTTPResourceServerTLSConfig{
			CACertFile:     caPath,
			ClientCertFile: clientCertPath,
			ClientKeyFile:  clientKeyPath,
		},
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := manager.CheckAccess(); err != nil {
		t.Fatalf("CheckAccess: %v", err)
	}
}

func TestIgnoreCheckAccessError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: true},
		{name: "notFound", err: &HTTPError{StatusCode: http.StatusNotFound}, want: true},
		{name: "methodNotAllowed", err: &HTTPError{StatusCode: http.StatusMethodNotAllowed}, want: true},
		{name: "redirect", err: &HTTPError{StatusCode: http.StatusFound}, want: true},
		{name: "failure", err: &HTTPError{StatusCode: http.StatusBadRequest}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ignoreCheckAccessError(tc.err); got != tc.want {
				t.Fatalf("ignoreCheckAccessError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
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

func TestHTTPResourceServerManagerBuildURLKeepsBasePath(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://rundeck.example/api/45/",
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	out, err := manager.buildURL("/projects/", nil)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if out != "https://rundeck.example/api/45/projects/" {
		t.Fatalf("unexpected build url %q", out)
	}
}

func TestHTTPResourceServerManagerBuildURLKeepsBasePathWithoutTrailingSlash(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://rundeck.example/api/45",
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	out, err := manager.buildURL("/projects/", nil)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if out != "https://rundeck.example/api/45/projects/" {
		t.Fatalf("unexpected build url %q", out)
	}
}

func TestHTTPResourceServerManagerBuildURLRespectsAbsolutePath(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://rundeck.example/api/45/",
	})
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	out, err := manager.buildURL("https://rundeck.example/projects/foo", nil)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if out != "https://rundeck.example/projects/foo" {
		t.Fatalf("unexpected build url %q", out)
	}
}

func TestHTTPResourceServerManagerLoadOpenAPISpecReadsFile(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "spec.json")
	content := `{"openapi":"3.0.0"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	data, err := manager.LoadOpenAPISpec(path)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected spec data %q", string(data))
	}
}

func TestHTTPResourceServerManagerLoadOpenAPISpecFetchesURL(t *testing.T) {
	content := `{"openapi":"3.0.0"}`
	specURL := "http://example.com/openapi.json"
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
	})
	manager.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("unexpected method %s", req.Method)
			}
			if req.URL.String() != specURL {
				t.Fatalf("unexpected url %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(content)),
			}, nil
		}),
	}

	data, err := manager.LoadOpenAPISpec(specURL)
	if err != nil {
		t.Fatalf("LoadOpenAPISpec: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected spec data %q", string(data))
	}
}

func TestHTTPResourceServerManagerLoadOpenAPISpecFileErrorDoesNotCallHTTP(t *testing.T) {
	manager := NewHTTPResourceServerManager(&HTTPResourceServerConfig{
		BaseURL: "https://example.com",
	})
	manager.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request %s %s", req.Method, req.URL.Path)
			return nil, nil
		}),
	}

	path := filepath.Join(t.TempDir(), "missing.json")
	_, err := manager.LoadOpenAPISpec(path)
	if err == nil {
		t.Fatalf("expected error when spec file is missing")
	}
	if !strings.Contains(err.Error(), "failed to read openapi file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func writeTestCertificateFiles(t *testing.T, dir string) (string, string, []byte) {
	t.Helper()

	certPEM, keyPEM := generateTestCertificate(t)
	certPath := filepath.Join(dir, "client-cert.pem")
	keyPath := filepath.Join(dir, "client-key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	return certPath, keyPath, certPEM
}

func generateTestCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}

func generateCACertificate(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key pair: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func generateSignedCertificate(t *testing.T, caCert *x509.Certificate, caKey *rsa.PrivateKey, template x509.Certificate) ([]byte, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	if template.SerialNumber == nil {
		template.SerialNumber = big.NewInt(time.Now().UnixNano())
	}
	template.BasicConstraintsValid = true

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}
