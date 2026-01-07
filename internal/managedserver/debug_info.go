package managedserver

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type HTTPRequestDebugInfo struct {
	Method  string
	URL     string
	Headers []string
	Body    string
}

type HTTPResponseDebugInfo struct {
	StatusCode int
	StatusText string
	Headers    []string
	Body       string
}

type HTTPInteraction struct {
	Request  *HTTPRequestDebugInfo
	Response *HTTPResponseDebugInfo
}

type ServerDebugInfo struct {
	Type                  string
	BaseURL               string
	AuthMethod            string
	TLSInsecureSkipVerify *bool
	DefaultHeaders        []string
	OpenAPI               string
	LastRequest           *HTTPRequestDebugInfo
	LastResponse          *HTTPResponseDebugInfo
	Interactions          []HTTPInteraction
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
		for key, value := range m.config.DefaultHeaders {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			info.DefaultHeaders = append(info.DefaultHeaders, fmt.Sprintf("%s: %s", key, value))
		}
		sort.Strings(info.DefaultHeaders)
	}
	info.OpenAPI = strings.TrimSpace(m.config.OpenAPI)

	m.debugMu.Lock()
	if m.lastRequest != nil {
		info.LastRequest = copyHTTPRequestDebugInfo(m.lastRequest)
	}
	if m.lastResponse != nil {
		info.LastResponse = copyHTTPResponseDebugInfo(m.lastResponse)
	}
	for _, interaction := range m.interactions {
		info.Interactions = append(info.Interactions, copyHTTPInteraction(interaction))
	}
	m.debugMu.Unlock()

	return info
}

func copyHTTPRequestDebugInfo(src *HTTPRequestDebugInfo) *HTTPRequestDebugInfo {
	if src == nil {
		return nil
	}
	return &HTTPRequestDebugInfo{
		Method:  src.Method,
		URL:     src.URL,
		Headers: append([]string(nil), src.Headers...),
		Body:    src.Body,
	}
}

func copyHTTPResponseDebugInfo(src *HTTPResponseDebugInfo) *HTTPResponseDebugInfo {
	if src == nil {
		return nil
	}
	return &HTTPResponseDebugInfo{
		StatusCode: src.StatusCode,
		StatusText: src.StatusText,
		Headers:    append([]string(nil), src.Headers...),
		Body:       src.Body,
	}
}

func copyHTTPInteraction(src HTTPInteraction) HTTPInteraction {
	return HTTPInteraction{
		Request:  copyHTTPRequestDebugInfo(src.Request),
		Response: copyHTTPResponseDebugInfo(src.Response),
	}
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
