package managedserver

import (
	"net/url"
	"sort"
	"strings"
)

type ServerDebugInfo struct {
	Type                  string
	BaseURL               string
	AuthMethod            string
	TLSInsecureSkipVerify *bool
	DefaultHeaders        []string
	OpenAPI               string
}

func (m *HTTPResourceServerManager) DebugInfo() ServerDebugInfo {
	info := ServerDebugInfo{Type: "http"}
	if m == nil || m.config == nil {
		return info
	}

	info.BaseURL = sanitizeURL(m.config.BaseURL)
	info.AuthMethod = authMethodLabel(m.config.Auth)
	if m.config.TLS != nil {
		value := m.config.TLS.InsecureSkipVerify
		info.TLSInsecureSkipVerify = &value
	}
	if len(m.config.DefaultHeaders) > 0 {
		for key := range m.config.DefaultHeaders {
			key = strings.TrimSpace(key)
			if key != "" {
				info.DefaultHeaders = append(info.DefaultHeaders, key)
			}
		}
		sort.Strings(info.DefaultHeaders)
	}
	info.OpenAPI = strings.TrimSpace(m.config.OpenAPI)

	return info
}

func authMethodLabel(cfg *HTTPResourceServerAuthConfig) string {
	if cfg == nil {
		return "none"
	}
	if cfg.OAuth2 != nil {
		return "oauth2"
	}
	if cfg.CustomHeader != nil {
		return "custom_header"
	}
	if cfg.BearerToken != nil {
		return "bearer_token"
	}
	if cfg.BasicAuth != nil {
		return "basic_auth"
	}
	return "none"
}

func sanitizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
