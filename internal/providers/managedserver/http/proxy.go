package http

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"golang.org/x/net/http/httpproxy"
)

func buildProxyFunc(proxyConfig *config.HTTPProxy) (func(*http.Request) (*url.URL, error), error) {
	if proxyConfig == nil {
		return nil, nil
	}

	httpURL, err := parseProxyURL("resource-server.http.proxy.http-url", proxyConfig.HTTPURL)
	if err != nil {
		return nil, err
	}
	httpsURL, err := parseProxyURL("resource-server.http.proxy.https-url", proxyConfig.HTTPSURL)
	if err != nil {
		return nil, err
	}

	if httpURL == nil && httpsURL == nil {
		return nil, faults.NewValidationError("resource-server.http.proxy must define at least one of http-url or https-url", nil)
	}

	auth := proxyConfig.Auth
	if auth != nil {
		username := strings.TrimSpace(auth.Username)
		password := strings.TrimSpace(auth.Password)
		if username == "" || password == "" {
			return nil, faults.NewValidationError("resource-server.http.proxy.auth requires username and password", nil)
		}

		var authErr error
		httpURL, authErr = applyProxyAuth(httpURL, username, password)
		if authErr != nil {
			return nil, authErr
		}
		httpsURL, authErr = applyProxyAuth(httpsURL, username, password)
		if authErr != nil {
			return nil, authErr
		}
	}

	httpProxyValue := ""
	httpsProxyValue := ""
	if httpURL != nil {
		httpProxyValue = httpURL.String()
	}
	if httpsURL != nil {
		httpsProxyValue = httpsURL.String()
	}

	resolver := (&httpproxy.Config{
		HTTPProxy:  httpProxyValue,
		HTTPSProxy: httpsProxyValue,
		NoProxy:    strings.TrimSpace(proxyConfig.NoProxy),
	}).ProxyFunc()

	return func(request *http.Request) (*url.URL, error) {
		if request == nil || request.URL == nil {
			return nil, nil
		}
		return resolver(request.URL)
	}, nil
}

func parseProxyURL(field string, raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, faults.NewValidationError(field+" is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, faults.NewValidationError(field+" must use http or https", nil)
	}
	if parsed.Host == "" {
		return nil, faults.NewValidationError(field+" host is required", nil)
	}

	return parsed, nil
}

func applyProxyAuth(proxyURL *url.URL, username string, password string) (*url.URL, error) {
	if proxyURL == nil {
		return nil, nil
	}

	if proxyURL.User != nil {
		return nil, faults.NewValidationError(
			"resource-server.http.proxy.auth cannot be combined with credentials embedded in proxy URL",
			nil,
		)
	}

	clone := *proxyURL
	clone.User = url.UserPassword(username, password)
	return &clone, nil
}
