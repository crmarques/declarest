package proxy

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	"golang.org/x/net/http/httpproxy"
)

// Config holds parsed proxy URLs for HTTP and HTTPS transports plus no-proxy rules.
type Config struct {
	HTTP        *url.URL
	HTTPS       *url.URL
	NoProxy     string
	fieldPrefix string
	auth        *config.ProxyAuth
	runtime     *promptauth.Runtime
}

var environmentKeys = []string{
	"HTTP_PROXY",
	"http_proxy",
	"HTTPS_PROXY",
	"https_proxy",
	"NO_PROXY",
	"no_proxy",
}

// Build parses the configuration and returns canonical proxy values.
func Build(fieldPrefix string, proxy *config.HTTPProxy) (Config, error) {
	return build(fieldPrefix, proxy, nil)
}

// BuildWithRuntime parses proxy settings and preserves prompt-auth resolution
// until runtime request execution.
func BuildWithRuntime(fieldPrefix string, proxy *config.HTTPProxy, runtime *promptauth.Runtime) (Config, error) {
	return build(fieldPrefix, proxy, runtime)
}

func build(fieldPrefix string, proxy *config.HTTPProxy, runtime *promptauth.Runtime) (Config, error) {
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

	auth, err := cloneProxyAuth(proxy.Auth)
	if err != nil {
		return Config{}, err
	}
	if auth != nil {
		if httpURL != nil && httpURL.User != nil {
			return Config{}, faults.NewValidationError(fieldPrefix+".auth cannot be combined with credentials embedded in proxy URL", nil)
		}
		if httpsURL != nil && httpsURL.User != nil {
			return Config{}, faults.NewValidationError(fieldPrefix+".auth cannot be combined with credentials embedded in proxy URL", nil)
		}
		if auth.Prompt == nil {
			if httpURL != nil {
				httpURL, err = applyProxyAuth(fieldPrefix+".auth", httpURL, auth.Username, auth.Password)
				if err != nil {
					return Config{}, err
				}
			}
			if httpsURL != nil {
				httpsURL, err = applyProxyAuth(fieldPrefix+".auth", httpsURL, auth.Username, auth.Password)
				if err != nil {
					return Config{}, err
				}
			}
			auth = nil
		}
	}

	return Config{
		HTTP:        httpURL,
		HTTPS:       httpsURL,
		NoProxy:     strings.TrimSpace(proxy.NoProxy),
		fieldPrefix: fieldPrefix,
		auth:        auth,
		runtime:     runtime,
	}, nil
}

// Resolve merges process proxy environment variables with the configured proxy
// block. Explicit empty proxy blocks disable both inherited and environment
// proxy settings.
func Resolve(fieldPrefix string, proxy *config.HTTPProxy) (Config, bool, error) {
	return resolve(fieldPrefix, proxy, nil)
}

// ResolveWithRuntime merges process proxy environment variables with the
// configured proxy block while keeping prompt-auth resolution lazy.
func ResolveWithRuntime(fieldPrefix string, proxy *config.HTTPProxy, runtime *promptauth.Runtime) (Config, bool, error) {
	return resolve(fieldPrefix, proxy, runtime)
}

func resolve(fieldPrefix string, proxy *config.HTTPProxy, runtime *promptauth.Runtime) (Config, bool, error) {
	if IsExplicitDisable(proxy) {
		return Config{}, true, nil
	}

	merged := Merge(FromEnvironment(), proxy)
	if merged == nil {
		return Config{}, false, nil
	}

	cfg, err := BuildWithRuntime(fieldPrefix, merged, runtime)
	if err != nil {
		return Config{}, false, err
	}
	return cfg, false, nil
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
		value, err := resolver(req.URL)
		if err != nil || value == nil {
			return value, err
		}
		return cfg.urlWithAuth(req.Context(), value)
	}
}

// Env returns proxy-related environment variables following http_proxy conventions.
func (cfg Config) Env(ctx context.Context) (map[string]string, error) {
	env := map[string]string{}
	if cfg.HTTP != nil {
		httpURL, err := cfg.urlWithAuth(ctx, cfg.HTTP)
		if err != nil {
			return nil, err
		}
		httpValue := httpURL.String()
		env["HTTP_PROXY"] = httpValue
		env["http_proxy"] = httpValue
	}
	if cfg.HTTPS != nil {
		httpsURL, err := cfg.urlWithAuth(ctx, cfg.HTTPS)
		if err != nil {
			return nil, err
		}
		httpsValue := httpsURL.String()
		env["HTTPS_PROXY"] = httpsValue
		env["https_proxy"] = httpsValue
	}
	if cfg.NoProxy != "" {
		env["NO_PROXY"] = cfg.NoProxy
		env["no_proxy"] = cfg.NoProxy
	}
	return env, nil
}

// EnvironmentKeys returns the proxy-related environment variable names used by
// the runtime.
func EnvironmentKeys() []string {
	keys := make([]string, len(environmentKeys))
	copy(keys, environmentKeys)
	return keys
}

// FromEnvironment returns the process proxy environment as a proxy block.
func FromEnvironment() *config.HTTPProxy {
	httpURL, hasHTTP := firstEnvValue("HTTP_PROXY", "http_proxy")
	httpsURL, hasHTTPS := firstEnvValue("HTTPS_PROXY", "https_proxy")
	noProxy, hasNoProxy := firstEnvValue("NO_PROXY", "no_proxy")
	if !hasHTTP && !hasHTTPS && !hasNoProxy {
		return nil
	}

	proxy := &config.HTTPProxy{
		HTTPURL:  httpURL,
		HTTPSURL: httpsURL,
		NoProxy:  noProxy,
	}
	if IsExplicitDisable(proxy) {
		return nil
	}
	return proxy
}

// Merge overlays override onto base field by field.
func Merge(base *config.HTTPProxy, override *config.HTTPProxy) *config.HTTPProxy {
	if IsExplicitDisable(override) {
		return &config.HTTPProxy{}
	}
	if base == nil && override == nil {
		return nil
	}

	merged := Clone(base)
	if merged == nil {
		merged = &config.HTTPProxy{}
	}
	if override == nil {
		return merged
	}

	overrideHTTPURL := strings.TrimSpace(override.HTTPURL)
	overrideHTTPSURL := strings.TrimSpace(override.HTTPSURL)
	if overrideHTTPURL != "" {
		merged.HTTPURL = overrideHTTPURL
	}
	if overrideHTTPSURL != "" {
		merged.HTTPSURL = overrideHTTPSURL
	}
	if value := strings.TrimSpace(override.NoProxy); value != "" {
		merged.NoProxy = value
	}
	if override.Auth != nil {
		if overrideHTTPURL == "" {
			merged.HTTPURL = stripProxyURLUserInfo(merged.HTTPURL)
		}
		if overrideHTTPSURL == "" {
			merged.HTTPSURL = stripProxyURLUserInfo(merged.HTTPSURL)
		}
		merged.Auth = &config.ProxyAuth{
			Username: strings.TrimSpace(override.Auth.Username),
			Password: strings.TrimSpace(override.Auth.Password),
		}
		if override.Auth.Prompt != nil {
			prompt := *override.Auth.Prompt
			merged.Auth.Prompt = &prompt
		}
	}

	return merged
}

// Clone duplicates the provided HTTP proxy configuration.
func Clone(proxy *config.HTTPProxy) *config.HTTPProxy {
	if proxy == nil {
		return nil
	}
	cloned := *proxy
	if proxy.Auth != nil {
		auth := *proxy.Auth
		if proxy.Auth.Prompt != nil {
			prompt := *proxy.Auth.Prompt
			auth.Prompt = &prompt
		}
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
		if strings.TrimSpace(proxy.Auth.Username) != "" || strings.TrimSpace(proxy.Auth.Password) != "" || proxy.Auth.Prompt != nil {
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
		if (a.Auth.Prompt == nil) != (b.Auth.Prompt == nil) {
			return false
		}
		if a.Auth.Prompt != nil && a.Auth.Prompt.KeepCredentialsForSession != b.Auth.Prompt.KeepCredentialsForSession {
			return false
		}
	}
	return true
}

func firstEnvValue(keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		return value, true
	}
	return "", false
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

func cloneProxyAuth(auth *config.ProxyAuth) (*config.ProxyAuth, error) {
	if auth == nil {
		return nil, nil
	}

	cloned := &config.ProxyAuth{
		Username: strings.TrimSpace(auth.Username),
		Password: strings.TrimSpace(auth.Password),
	}
	if auth.Prompt != nil {
		prompt := *auth.Prompt
		cloned.Prompt = &prompt
	}

	hasUserPass := cloned.Username != "" || cloned.Password != ""
	hasPrompt := cloned.Prompt != nil
	if countSet(hasUserPass, hasPrompt) != 1 {
		return nil, faults.NewValidationError("proxy auth must define either username/password or prompt", nil)
	}
	if hasPrompt {
		return cloned, nil
	}
	if cloned.Username == "" || cloned.Password == "" {
		return nil, faults.NewValidationError("proxy auth requires username and password", nil)
	}
	return cloned, nil
}

func (cfg Config) urlWithAuth(ctx context.Context, base *url.URL) (*url.URL, error) {
	if base == nil || cfg.auth == nil {
		return base, nil
	}

	if cfg.auth.Prompt == nil {
		return applyProxyAuth(cfg.fieldPrefix+".auth", base, cfg.auth.Username, cfg.auth.Password)
	}
	if cfg.runtime == nil {
		return nil, faults.NewValidationError(cfg.fieldPrefix+".auth.prompt requires prompt auth runtime support", nil)
	}

	targetKey, ok := proxyTargetKey(cfg.fieldPrefix)
	if !ok {
		return nil, faults.NewValidationError(cfg.fieldPrefix+".auth.prompt is not supported", nil)
	}
	creds, err := cfg.runtime.Resolve(ctx, targetKey, cfg.auth.Prompt.KeepCredentialsForSession)
	if err != nil {
		return nil, err
	}
	return applyProxyAuth(cfg.fieldPrefix+".auth", base, creds.Username, creds.Password)
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

func stripProxyURLUserInfo(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	if parsed.User == nil {
		return value
	}

	clone := *parsed
	clone.User = nil
	return clone.String()
}

func proxyTargetKey(fieldPrefix string) (string, bool) {
	switch strings.TrimSpace(fieldPrefix) {
	case "managed-server.http.proxy", "managedServer.http.proxy":
		return promptauth.TargetManagedServerHTTPProxyAuth, true
	case "repository.git.remote.proxy":
		return promptauth.TargetRepositoryGitRemoteProxyAuth, true
	case "secret-store.vault.proxy", "secretStore.vault.proxy":
		return promptauth.TargetSecretStoreVaultProxyAuth, true
	case "metadata.proxy":
		return promptauth.TargetMetadataProxyAuth, true
	default:
		return "", false
	}
}

func countSet(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
