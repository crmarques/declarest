package proxy

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"golang.org/x/net/http/httpproxy"
)

// Config holds parsed proxy URLs for HTTP and HTTPS transports plus no-proxy rules.
type Config struct {
	HTTP    *url.URL
	HTTPS   *url.URL
	NoProxy string
}

// Build parses the configuration and returns canonical proxy values.
func Build(fieldPrefix string, proxy *config.HTTPProxy) (Config, error) {
	if proxy == nil {
		return Config{}, nil
	}

	httpURL, err := parseProxyURL(fieldPrefix+".http-url", proxy.HTTPURL)
	if err != nil {
		return Config{}, err
	}
	httpsURL, err := parseProxyURL(fieldPrefix+".https-url", proxy.HTTPSURL)
	if err != nil {
		return Config{}, err
	}

	noProxy := strings.TrimSpace(proxy.NoProxy)

	if proxy.Auth != nil {
		username := strings.TrimSpace(proxy.Auth.Username)
		password := strings.TrimSpace(proxy.Auth.Password)
		if username == "" || password == "" {
			return Config{}, faults.NewValidationError(fieldPrefix+".auth requires username and password", nil)
		}
		if httpURL != nil {
			httpURL, err = applyProxyAuth(fieldPrefix+".auth", httpURL, username, password)
			if err != nil {
				return Config{}, err
			}
		}
		if httpsURL != nil {
			httpsURL, err = applyProxyAuth(fieldPrefix+".auth", httpsURL, username, password)
			if err != nil {
				return Config{}, err
			}
		}
	}

	cfg := Config{
		HTTP:    httpURL,
		HTTPS:   httpsURL,
		NoProxy: noProxy,
	}

	return cfg, nil
}

// HasProxy returns true when either HTTP or HTTPS proxy URL is configured.
func (cfg Config) HasProxy() bool {
	return cfg.HTTP != nil || cfg.HTTPS != nil
}

// Resolver returns a proxy resolver suitable for HTTP transports.
func (cfg Config) Resolver() func(*http.Request) (*url.URL, error) {
	if !cfg.HasProxy() && cfg.NoProxy == "" {
		return nil
	}
	resolver := (&httpproxy.Config{
		HTTPProxy:  proxyURLString(cfg.HTTP),
		HTTPSProxy: proxyURLString(cfg.HTTPS),
		NoProxy:    cfg.NoProxy,
	}).ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		if req == nil || req.URL == nil {
			return nil, nil
		}
		return resolver(req.URL)
	}
}

// Env returns proxy-related environment variables following http_proxy conventions.
func (cfg Config) Env() map[string]string {
	env := map[string]string{}
	if cfg.HTTP != nil {
		httpValue := cfg.HTTP.String()
		env["HTTP_PROXY"] = httpValue
		env["http_proxy"] = httpValue
	}
	if cfg.HTTPS != nil {
		httpsValue := cfg.HTTPS.String()
		env["HTTPS_PROXY"] = httpsValue
		env["https_proxy"] = httpsValue
	}
	if cfg.NoProxy != "" {
		env["NO_PROXY"] = cfg.NoProxy
		env["no_proxy"] = cfg.NoProxy
	}
	return env
}

// Clone duplicates the provided HTTP proxy configuration.
func Clone(proxy *config.HTTPProxy) *config.HTTPProxy {
	if proxy == nil {
		return nil
	}
	cloned := *proxy
	if proxy.Auth != nil {
		auth := *proxy.Auth
		cloned.Auth = &auth
	}
	return &cloned
}

// HasURLs returns true when the proxy contains at least one URL field.
func HasURLs(proxy *config.HTTPProxy) bool {
	if proxy == nil {
		return false
	}
	if strings.TrimSpace(proxy.HTTPURL) != "" {
		return true
	}
	if strings.TrimSpace(proxy.HTTPSURL) != "" {
		return true
	}
	return false
}

// IsExplicitDisable returns true when the proxy block is present but contains no actionable values.
func IsExplicitDisable(proxy *config.HTTPProxy) bool {
	if proxy == nil {
		return false
	}
	if HasURLs(proxy) {
		return false
	}
	if strings.TrimSpace(proxy.NoProxy) != "" {
		return false
	}
	if proxy.Auth != nil {
		if strings.TrimSpace(proxy.Auth.Username) != "" || strings.TrimSpace(proxy.Auth.Password) != "" {
			return false
		}
	}
	return true
}

// Equal compares two proxy blocks for semaphore equality.
func Equal(a, b *config.HTTPProxy) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if strings.TrimSpace(a.HTTPURL) != strings.TrimSpace(b.HTTPURL) {
		return false
	}
	if strings.TrimSpace(a.HTTPSURL) != strings.TrimSpace(b.HTTPSURL) {
		return false
	}
	if strings.TrimSpace(a.NoProxy) != strings.TrimSpace(b.NoProxy) {
		return false
	}
	if (a.Auth == nil) != (b.Auth == nil) {
		return false
	}
	if a.Auth != nil {
		if strings.TrimSpace(a.Auth.Username) != strings.TrimSpace(b.Auth.Username) {
			return false
		}
		if strings.TrimSpace(a.Auth.Password) != strings.TrimSpace(b.Auth.Password) {
			return false
		}
	}
	return true
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
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, faults.NewValidationError(field+" must use http or https", nil)
	}
	if parsed.Host == "" {
		return nil, faults.NewValidationError(field+" host is required", nil)
	}
	return parsed, nil
}

func applyProxyAuth(fieldPrefix string, proxyURL *url.URL, username, password string) (*url.URL, error) {
	if proxyURL == nil {
		return nil, nil
	}

	if proxyURL.User != nil {
		return nil, faults.NewValidationError(fieldPrefix+" cannot be combined with credentials embedded in proxy URL", nil)
	}

	clone := *proxyURL
	clone.User = url.UserPassword(username, password)
	return &clone, nil
}

func proxyURLString(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}
