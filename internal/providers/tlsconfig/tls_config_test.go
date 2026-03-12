package tlsconfig

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestBuildTLSConfigNilSettingsReturnsNil(t *testing.T) {
	t.Parallel()

	tlsConfig, err := BuildTLSConfig(nil, "managed-server.http")
	if err != nil {
		t.Fatalf("BuildTLSConfig returned error: %v", err)
	}
	if tlsConfig != nil {
		t.Fatalf("expected nil tls config, got %#v", tlsConfig)
	}
}

func TestBuildTLSConfigRejectsMissingCAFile(t *testing.T) {
	t.Parallel()

	_, err := BuildTLSConfig(&config.TLS{CACertFile: "/tmp/does-not-exist.pem"}, "managed-server.http")
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBuildTLSConfigRejectsInvalidCAPEM(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	caFile := filepath.Join(tmp, "ca.pem")
	if err := os.WriteFile(caFile, []byte("not-pem"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := BuildTLSConfig(&config.TLS{CACertFile: caFile}, "managed-server.http")
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBuildTLSConfigRequiresClientCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	_, err := BuildTLSConfig(&config.TLS{ClientCertFile: "/tmp/client.pem"}, "managed-server.http")
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBuildTLSConfigRejectsInvalidClientCertificatePair(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	certFile := filepath.Join(tmp, "client.pem")
	keyFile := filepath.Join(tmp, "client.key")
	if err := os.WriteFile(certFile, []byte("invalid-cert"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("invalid-key"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := BuildTLSConfig(&config.TLS{
		ClientCertFile: certFile,
		ClientKeyFile:  keyFile,
	}, "managed-server.http")
	if !faults.IsCategory(err, faults.ValidationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBuildTLSConfigAppliesBaseTLSSettings(t *testing.T) {
	t.Parallel()

	tlsConfig, err := BuildTLSConfig(&config.TLS{InsecureSkipVerify: true}, "managed-server.http")
	if err != nil {
		t.Fatalf("BuildTLSConfig returned error: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected MinVersion TLS1.2, got %v", tlsConfig.MinVersion)
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be propagated")
	}
}
