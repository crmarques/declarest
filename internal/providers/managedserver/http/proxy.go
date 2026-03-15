package http

import (
	"net/http"
	"net/url"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/promptauth"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

func buildProxyFunc(proxyConfig *config.HTTPProxy, runtime *promptauth.Runtime) (func(*http.Request) (*url.URL, error), error) {
	cfg, disabled, err := proxyhelper.ResolveWithRuntime("managed-server.http.proxy", proxyConfig, runtime)
	if err != nil {
		return nil, err
	}
	if disabled {
		return nil, nil
	}
	return cfg.Resolver(), nil
}
