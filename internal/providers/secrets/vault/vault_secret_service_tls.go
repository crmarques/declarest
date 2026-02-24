package vault

import (
	"crypto/tls"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/providers/shared/tlsconfig"
)

func buildTLSConfig(tlsSettings *config.TLS) (*tls.Config, error) {
	return tlsconfig.BuildTLSConfig(tlsSettings, "secret-store.vault")
}
