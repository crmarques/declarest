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

package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/debugctx"
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

func (g *Client) doRequest(ctx context.Context, purpose string, request *http.Request) (*http.Response, error) {
	resolvedURL := resolveURLForDebug(ctx, request.URL)

	// Level 2: HTTP request summary
	debugctx.Detailf(
		ctx,
		"http request purpose=%q method=%q url=%q",
		purpose,
		request.Method,
		resolvedURL,
	)

	// Level 3: full request details including TLS
	debugctx.Printf(
		ctx,
		"http request purpose=%q method=%q url=%q tls_enabled=%t mtls_enabled=%t tls_insecure_skip_verify=%t tls_ca_cert_file=%q tls_client_cert_file=%q tls_client_key_file=%q",
		purpose,
		request.Method,
		resolvedURL,
		g.tlsDebug.enabled,
		g.tlsDebug.mTLSEnabled(),
		g.tlsDebug.insecureSkipVerify,
		g.tlsDebug.caCertFile,
		g.tlsDebug.clientCertFile,
		g.tlsDebug.clientKeyFile,
	)

	// Level 3: request headers
	debugRequestHeaders(ctx, request, g.auth)

	start := time.Now()

	invoke := func() (*http.Response, error) {
		return g.client.Do(request)
	}
	response, err := g.executeWithThrottle(ctx, purpose, request, invoke)
	elapsed := time.Since(start)

	if err != nil {
		// Level 2: request failure with timing
		debugctx.Detailf(
			ctx,
			"http request failed purpose=%q method=%q url=%q elapsed=%s error=%v",
			purpose,
			request.Method,
			resolvedURL,
			elapsed.Truncate(time.Millisecond),
			err,
		)
		return nil, err
	}

	// Level 2: response summary with timing
	debugctx.Detailf(
		ctx,
		"http response purpose=%q method=%q url=%q status=%d elapsed=%s",
		purpose,
		request.Method,
		resolvedURL,
		response.StatusCode,
		elapsed.Truncate(time.Millisecond),
	)

	// Level 3: response headers
	debugResponseHeaders(ctx, response)

	return response, nil
}

func (g *Client) executeWithThrottle(
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
			fmt.Sprintf("managed-service request canceled while waiting for throttling (%s %s %s)", purpose, request.Method, request.URL.Path),
			ctxErr,
		)
	}
	return nil, err
}

func debugRequestHeaders(ctx context.Context, request *http.Request, auth authConfig) {
	if debugctx.Level(ctx) < 3 || request == nil {
		return
	}

	headers := request.Header.Clone()
	if !debugctx.Insecure(ctx) {
		for key := range headers {
			if !auth.shouldRedactHeader(key) {
				continue
			}
			for _, value := range headers.Values(key) {
				headers.Del(key)
				headers.Add(key, redactHeaderValue(value))
			}
		}
	}

	keys := sortedHeaderKeys(headers)
	for _, key := range keys {
		for _, value := range headers.Values(key) {
			debugctx.Printf(ctx, "http request header %s: %s", key, value)
		}
	}
}

func debugResponseHeaders(ctx context.Context, response *http.Response) {
	if debugctx.Level(ctx) < 3 || response == nil {
		return
	}

	keys := sortedHeaderKeys(response.Header)
	for _, key := range keys {
		for _, value := range response.Header.Values(key) {
			debugctx.Printf(ctx, "http response header %s: %s", key, value)
		}
	}
}

func sortedHeaderKeys(headers http.Header) []string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func redactHeaderValue(value string) string {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) == 2 {
		return parts[0] + " <redacted>"
	}
	return "<redacted>"
}

// resolveURLForDebug returns the URL string for debug output.
// When --verbose-insecure is enabled, the full URL (with credentials and query params) is shown.
// Otherwise, user info and query parameter values are redacted.
func resolveURLForDebug(ctx context.Context, value *url.URL) string {
	if debugctx.Insecure(ctx) {
		if value == nil {
			return ""
		}
		return value.String()
	}
	return redactURLForDebug(value)
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
