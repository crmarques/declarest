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
			return nil, faults.NewValidationError(fmt.Sprintf("%s.tls.ca-cert-file could not be read", scope), err)
		}

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caBytes); !ok {
			return nil, faults.NewValidationError(fmt.Sprintf("%s.tls.ca-cert-file is not valid PEM", scope), nil)
		}
		tlsConfig.RootCAs = pool
	}

	clientCertFile := strings.TrimSpace(tlsSettings.ClientCertFile)
	clientKeyFile := strings.TrimSpace(tlsSettings.ClientKeyFile)
	if (clientCertFile == "") != (clientKeyFile == "") {
		return nil, faults.NewValidationError(
			fmt.Sprintf("%s.tls requires both client-cert-file and client-key-file", scope),
			nil,
		)
	}

	if clientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, faults.NewValidationError(
				fmt.Sprintf("%s.tls client certificate pair is invalid", scope),
				err,
			)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return tlsConfig, nil
}
