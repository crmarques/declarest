package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
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

func (g *HTTPManagedServerClient) doRequest(ctx context.Context, purpose string, request *http.Request) (*http.Response, error) {
	debugctx.Printf(
		ctx,
		"http request purpose=%q method=%q url=%q tls_enabled=%t mtls_enabled=%t tls_insecure_skip_verify=%t tls_ca_cert_file=%q tls_client_cert_file=%q tls_client_key_file=%q",
		purpose,
		request.Method,
		redactURLForDebug(request.URL),
		g.tlsDebug.enabled,
		g.tlsDebug.mTLSEnabled(),
		g.tlsDebug.insecureSkipVerify,
		g.tlsDebug.caCertFile,
		g.tlsDebug.clientCertFile,
		g.tlsDebug.clientKeyFile,
	)

	invoke := func() (*http.Response, error) {
		return g.client.Do(request)
	}
	response, err := g.executeWithThrottle(ctx, purpose, request, invoke)
	if err != nil {
		debugctx.Printf(
			ctx,
			"http request failed purpose=%q method=%q url=%q error=%v",
			purpose,
			request.Method,
			redactURLForDebug(request.URL),
			err,
		)
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"http response purpose=%q method=%q url=%q status=%d",
		purpose,
		request.Method,
		redactURLForDebug(request.URL),
		response.StatusCode,
	)
	return response, nil
}

func (g *HTTPManagedServerClient) executeWithThrottle(
	ctx context.Context,
	purpose string,
	request *http.Request,
	invoke func() (*http.Response, error),
) (*http.Response, error) {
	if g == nil || g.throttle == nil {
		return invoke()
	}
	response, err := g.throttle.execute(ctx, invoke)
	if err == nil {
		return response, nil
	}
	if faults.IsCategory(err, faults.ConflictError) || faults.IsCategory(err, faults.TransportError) {
		return nil, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, faults.NewTypedError(
			faults.TransportError,
			fmt.Sprintf("managed-server request canceled while waiting for throttling (%s %s %s)", purpose, request.Method, request.URL.Path),
			ctxErr,
		)
	}
	return nil, err
}

func redactURLForDebug(value *url.URL) string {
	if value == nil {
		return ""
	}

	cloned := *value
	cloned.User = nil

	query := cloned.Query()
	if len(query) > 0 {
		for key, values := range query {
			redacted := make([]string, len(values))
			for idx := range values {
				redacted[idx] = "<redacted>"
			}
			query[key] = redacted
		}
		cloned.RawQuery = query.Encode()
	}

	return cloned.String()
}
