package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
)

type tlsDebugInfo struct {
	enabled            bool
	insecureSkipVerify bool
	caCertFile         string
	clientCertFile     string
	clientKeyFile      string
}

func newTLSDebugInfo(tlsSettings *config.TLS) tlsDebugInfo {
	if tlsSettings == nil {
		return tlsDebugInfo{}
	}

	return tlsDebugInfo{
		enabled:            true,
		insecureSkipVerify: tlsSettings.InsecureSkipVerify,
		caCertFile:         strings.TrimSpace(tlsSettings.CACertFile),
		clientCertFile:     strings.TrimSpace(tlsSettings.ClientCertFile),
		clientKeyFile:      strings.TrimSpace(tlsSettings.ClientKeyFile),
	}
}

func (info tlsDebugInfo) mTLSEnabled() bool {
	return info.clientCertFile != "" && info.clientKeyFile != ""
}

func (g *HTTPResourceServerGateway) doRequest(ctx context.Context, purpose string, request *http.Request) (*http.Response, error) {
	debugctx.Printf(
		ctx,
		"http request purpose=%q method=%q url=%q tls_enabled=%t mtls_enabled=%t tls_insecure_skip_verify=%t tls_ca_cert_file=%q tls_client_cert_file=%q tls_client_key_file=%q",
		purpose,
		request.Method,
		request.URL.String(),
		g.tlsDebug.enabled,
		g.tlsDebug.mTLSEnabled(),
		g.tlsDebug.insecureSkipVerify,
		g.tlsDebug.caCertFile,
		g.tlsDebug.clientCertFile,
		g.tlsDebug.clientKeyFile,
	)

	response, err := g.client.Do(request)
	if err != nil {
		debugctx.Printf(
			ctx,
			"http request failed purpose=%q method=%q url=%q error=%v",
			purpose,
			request.Method,
			request.URL.String(),
			err,
		)
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"http response purpose=%q method=%q url=%q status=%d",
		purpose,
		request.Method,
		request.URL.String(),
		response.StatusCode,
	)
	return response, nil
}
