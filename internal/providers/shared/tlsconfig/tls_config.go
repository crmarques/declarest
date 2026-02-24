package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func BuildTLSConfig(tlsSettings *config.TLS, scope string) (*tls.Config, error) {
	if tlsSettings == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tlsSettings.InsecureSkipVerify,
	}

	if strings.TrimSpace(tlsSettings.CACertFile) != "" {
		caBytes, err := os.ReadFile(tlsSettings.CACertFile)
		if err != nil {
			return nil, validationError(fmt.Sprintf("%s.tls.ca-cert-file could not be read", scope), err)
		}

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caBytes); !ok {
			return nil, validationError(fmt.Sprintf("%s.tls.ca-cert-file is not valid PEM", scope), nil)
		}
		tlsConfig.RootCAs = pool
	}

	clientCertFile := strings.TrimSpace(tlsSettings.ClientCertFile)
	clientKeyFile := strings.TrimSpace(tlsSettings.ClientKeyFile)
	if (clientCertFile == "") != (clientKeyFile == "") {
		return nil, validationError(
			fmt.Sprintf("%s.tls requires both client-cert-file and client-key-file", scope),
			nil,
		)
	}

	if clientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, validationError(
				fmt.Sprintf("%s.tls client certificate pair is invalid", scope),
				err,
			)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return tlsConfig, nil
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}
