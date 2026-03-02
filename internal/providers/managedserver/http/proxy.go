package http

import (
	"net/http"
	"net/url"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

func buildProxyFunc(proxyConfig *config.HTTPProxy) (func(*http.Request) (*url.URL, error), error) {
	if proxyConfig == nil || proxyhelper.IsExplicitDisable(proxyConfig) {
		return nil, nil
	}

	if !proxyhelper.HasURLs(proxyConfig) {
		return nil, faults.NewValidationError("managed-server.http.proxy must define at least one of http-url or https-url", nil)
	}

	cfg, err := proxyhelper.Build("managed-server.http.proxy", proxyConfig)
	if err != nil {
		return nil, err
	}

	return cfg.Resolver(), nil
}
