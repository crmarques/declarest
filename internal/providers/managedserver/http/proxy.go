package http

import (
	"net/http"
	"net/url"

	"github.com/crmarques/declarest/config"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

func buildProxyFunc(proxyConfig *config.HTTPProxy) (func(*http.Request) (*url.URL, error), error) {
	cfg, disabled, err := proxyhelper.Resolve("managed-server.http.proxy", proxyConfig)
	if err != nil {
		return nil, err
	}
	if disabled {
		return nil, nil
	}
	return cfg.Resolver(), nil
}
