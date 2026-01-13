package managedserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"declarest/internal/resource"
)

const (
	defaultRequestTimeout  = 30 * time.Second
	defaultOAuthExpirySkew = 30 * time.Second
	maxDebugBodyCharacters = 1024
)

type HTTPResourceServerManager struct {
	config  *HTTPResourceServerConfig
	client  *http.Client
	baseURL *url.URL

	oauthMu      sync.Mutex
	oauthToken   *oauthToken
	debugMu      sync.Mutex
	lastRequest  *HTTPRequestDebugInfo
	lastResponse *HTTPResponseDebugInfo
	interactions []HTTPInteraction
}

func NewHTTPResourceServerManager(cfg *HTTPResourceServerConfig) *HTTPResourceServerManager {
	return &HTTPResourceServerManager{
		config: cfg,
	}
}

func (m *HTTPResourceServerManager) Init() error {
	if m == nil {
		return errors.New("http resource server manager is nil")
	}
	if m.config == nil {
		return errors.New("http resource server configuration is required")
	}

	rawBase := strings.TrimSpace(m.config.BaseURL)
	if rawBase == "" {
		return errors.New("http resource server base_url is required")
	}

	parsed, err := url.Parse(rawBase)
	if err != nil {
		return fmt.Errorf("invalid base_url %q: %w", rawBase, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("base_url %q must include scheme and host", rawBase)
	}
	m.baseURL = parsed

	tlsCfg := &tls.Config{}
	if m.config.TLS != nil && m.config.TLS.InsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	m.client = &http.Client{
		Transport: transport,
	}

	return nil
}

func (m *HTTPResourceServerManager) Close() error {
	if m == nil || m.client == nil {
		return nil
	}
	if tr, ok := m.client.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	return nil
}

func (m *HTTPResourceServerManager) CheckAccess() error {
	if err := m.Init(); err != nil {
		return err
	}
	if m.baseURL == nil {
		return errors.New("http manager base url is not configured")
	}

	spec := &HTTPRequestSpec{
		Method: http.MethodHead,
		Path:   m.baseURL.String(),
	}

	_, err := m.executeRequest(spec, defaultRequestTimeout, nil, false)
	if err == nil {
		return nil
	}
	if ignoreCheckAccessError(err) {
		return nil
	}
	return err
}

func ignoreCheckAccessError(err error) bool {
	if err == nil {
		return true
	}
	if IsNotFoundError(err) {
		return true
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusMethodNotAllowed {
			return true
		}
		if httpErr.StatusCode >= http.StatusMultipleChoices && httpErr.StatusCode < http.StatusBadRequest {
			return true
		}
	}
	return false
}

func (m *HTTPResourceServerManager) GetResource(custom RequestSpec) (resource.Resource, error) {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodGet, &custom)
	if err != nil {
		return resource.Resource{}, err
	}

	resp, err := m.doRequest(httpSpec, timeout, nil)
	if err != nil {
		return resource.Resource{}, err
	}

	res, err := resource.NewResourceFromJSON(resp.Body)
	if err != nil {
		return resource.Resource{}, fmt.Errorf("failed to decode response body: %w", err)
	}
	return res, nil
}

func (m *HTTPResourceServerManager) GetResourceCollection(custom RequestSpec) ([]resource.Resource, error) {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodGet, &custom)
	if err != nil {
		return nil, err
	}

	resp, err := m.doRequest(httpSpec, timeout, nil)
	if err != nil {
		return nil, err
	}

	var payload []any
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return []resource.Resource{}, nil
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode collection response: %w", err)
	}

	result := make([]resource.Resource, 0, len(payload))
	for _, entry := range payload {
		res, err := resource.NewResource(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize collection entry: %w", err)
		}
		result = append(result, res)
	}
	return result, nil
}

func (m *HTTPResourceServerManager) CreateResource(data resource.Resource, custom RequestSpec) error {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodPost, &custom)
	if err != nil {
		return err
	}

	body, err := m.encodeResource(data)
	if err != nil {
		return err
	}

	_, err = m.doRequest(httpSpec, timeout, body)
	return err
}

func (m *HTTPResourceServerManager) UpdateResource(data resource.Resource, custom RequestSpec) error {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodPut, &custom)
	if err != nil {
		return err
	}

	body, err := m.encodeResource(data)
	if err != nil {
		return err
	}

	_, err = m.doRequest(httpSpec, timeout, body)
	return err
}

func (m *HTTPResourceServerManager) DeleteResource(custom RequestSpec) error {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodDelete, &custom)
	if err != nil {
		return err
	}

	_, err = m.doRequest(httpSpec, timeout, nil)
	return err
}

func (m *HTTPResourceServerManager) ResourceExists(custom RequestSpec) (bool, error) {
	httpSpec, timeout, err := m.normalizeSpec(http.MethodHead, &custom)
	if err != nil {
		return false, err
	}

	resp, err := m.executeRequest(httpSpec, timeout, nil, false)
	if err != nil {
		if IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func (m *HTTPResourceServerManager) normalizeSpec(defaultMethod string, custom *RequestSpec) (*HTTPRequestSpec, time.Duration, error) {
	if m == nil {
		return nil, 0, errors.New("http manager is not initialized")
	}
	if m.client == nil {
		return nil, 0, errors.New("http manager is not initialized")
	}

	var timeout time.Duration
	var httpSpec *HTTPRequestSpec

	if custom != nil {
		timeout = custom.Timeout
		if custom.Kind != "" && custom.Kind != KindHTTP {
			return nil, 0, fmt.Errorf("request kind %q not supported", custom.Kind)
		}
		if custom.HTTP != nil {
			specCopy := *custom.HTTP
			httpSpec = &specCopy
		}
	}

	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	if httpSpec == nil {
		httpSpec = &HTTPRequestSpec{}
	}

	if strings.TrimSpace(httpSpec.Method) == "" {
		httpSpec.Method = defaultMethod
	}

	if httpSpec.Accept == "" && defaultMethod != http.MethodDelete && defaultMethod != http.MethodHead {
		httpSpec.Accept = m.defaultMimeFor(custom)
	}

	if httpSpec.ContentType == "" {
		httpSpec.ContentType = m.defaultMimeFor(custom)
	}

	if strings.TrimSpace(httpSpec.Path) == "" {
		return nil, 0, errors.New("http request path is required")
	}

	return httpSpec, timeout, nil
}

func (m *HTTPResourceServerManager) defaultMimeFor(spec *RequestSpec) string {
	if spec == nil || spec.HTTP == nil {
		return "application/json"
	}
	switch strings.ToLower(strings.TrimSpace(spec.HTTP.ContentType)) {
	case "application/x-yaml", "application/yaml", "text/yaml":
		return "application/x-yaml"
	case "application/json", "":
		return "application/json"
	default:
		return spec.HTTP.ContentType
	}
}

func (m *HTTPResourceServerManager) doRequest(spec *HTTPRequestSpec, timeout time.Duration, payload []byte) (*HTTPResponse, error) {
	return m.executeRequest(spec, timeout, payload, true)
}

func (m *HTTPResourceServerManager) ExecuteRequest(spec *HTTPRequestSpec, payload []byte) (*HTTPResponse, error) {
	return m.doRequest(spec, defaultRequestTimeout, payload)
}

func (m *HTTPResourceServerManager) executeRequest(spec *HTTPRequestSpec, timeout time.Duration, payload []byte, readBody bool) (*HTTPResponse, error) {
	if m.client == nil {
		return nil, errors.New("http manager is not initialized")
	}

	fullURL, err := m.buildURL(spec.Path, spec.Query)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, spec.Method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if spec.ContentType != "" {
		req.Header.Set("Content-Type", spec.ContentType)
	}
	if spec.Accept != "" {
		req.Header.Set("Accept", spec.Accept)
	}

	m.applyDefaultHeaders(req)
	applyCustomHeaders(req.Header, spec.Headers)

	if err := m.applyAuth(ctx, req); err != nil {
		return nil, err
	}
	m.recordRequest(req, payload)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	m.recordResponse(resp.StatusCode, resp.Header.Clone(), body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if !readBody {
			body = nil
		}
		return &HTTPResponse{
			StatusCode: resp.StatusCode,
			Body:       body,
			Header:     resp.Header.Clone(),
			URL:        fullURL,
		}, nil
	}

	return nil, &HTTPError{
		Method:     spec.Method,
		URL:        fullURL,
		StatusCode: resp.StatusCode,
		Body:       body,
	}
}

func (m *HTTPResourceServerManager) applyDefaultHeaders(req *http.Request) {
	if m.config == nil || len(m.config.DefaultHeaders) == 0 {
		return
	}
	for key, value := range m.config.DefaultHeaders {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if value == "" {
			continue
		}
		if req.Header.Get(key) == "" {
			req.Header.Set(key, value)
		}
	}
}

func applyCustomHeaders(dst http.Header, headers map[string][]string) {
	if len(headers) == 0 {
		return
	}
	for key, values := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func (m *HTTPResourceServerManager) buildURL(path string, query map[string][]string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("http request path is required")
	}

	var reqURL *url.URL
	effectivePath := path
	if strings.HasPrefix(path, "/") {
		effectivePath = strings.TrimPrefix(path, "/")
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil {
			return "", fmt.Errorf("invalid request path %q: %w", path, err)
		}
		reqURL = u
	} else {
		if m.baseURL == nil {
			return "", errors.New("http manager base url is not configured")
		}
		rel, err := url.Parse(effectivePath)
		if err != nil {
			return "", fmt.Errorf("invalid request path %q: %w", path, err)
		}
		base := *m.baseURL
		if base.Path != "" && !strings.HasSuffix(base.Path, "/") {
			base.Path = base.Path + "/"
		}
		reqURL = base.ResolveReference(rel)
	}

	if len(query) > 0 {
		values := reqURL.Query()
		for key, items := range query {
			for _, item := range items {
				values.Add(key, item)
			}
		}
		reqURL.RawQuery = values.Encode()
	}

	return reqURL.String(), nil
}

func (m *HTTPResourceServerManager) encodeResource(res resource.Resource) ([]byte, error) {
	payload, err := res.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to encode resource payload: %w", err)
	}
	return payload, nil
}

type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
	URL        string
}

func (m *HTTPResourceServerManager) recordRequest(req *http.Request, body []byte) {
	if m == nil || req == nil {
		return
	}
	headers := formatHeadersList(req.Header)
	m.debugMu.Lock()
	defer m.debugMu.Unlock()
	if len(m.interactions) == 0 || (m.interactions[len(m.interactions)-1].Request != nil && m.interactions[len(m.interactions)-1].Response != nil) {
		m.interactions = append(m.interactions, HTTPInteraction{})
	}
	if len(m.interactions) > 10 {
		m.interactions = m.interactions[len(m.interactions)-10:]
	}
	current := &m.interactions[len(m.interactions)-1]
	current.Request = &HTTPRequestDebugInfo{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    limitDebugString(string(body)),
	}
	m.lastRequest = current.Request
	if len(m.interactions) > 10 {
		m.interactions = m.interactions[len(m.interactions)-10:]
	}
}

func (m *HTTPResourceServerManager) recordResponse(status int, header http.Header, body []byte) {
	if m == nil {
		return
	}
	respInfo := &HTTPResponseDebugInfo{
		StatusCode: status,
		StatusText: http.StatusText(status),
		Headers:    formatHeadersList(header),
		Body:       limitDebugString(string(body)),
	}
	m.debugMu.Lock()
	defer m.debugMu.Unlock()
	if len(m.interactions) == 0 {
		m.interactions = append(m.interactions, HTTPInteraction{})
	}
	current := &m.interactions[len(m.interactions)-1]
	current.Response = respInfo
	m.lastResponse = respInfo
}

func formatHeadersList(header http.Header) []string {
	if len(header) == 0 {
		return nil
	}
	var lines []string
	for key, values := range header {
		for _, value := range values {
			lines = append(lines, fmt.Sprintf("%s: %s", key, value))
		}
	}
	sort.Strings(lines)
	return lines
}

func limitDebugString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= maxDebugBodyCharacters {
		return trimmed
	}
	return trimmed[:maxDebugBodyCharacters] + "... (truncated)"
}

func (m *HTTPResourceServerManager) applyAuth(ctx context.Context, req *http.Request) error {
	if m.config == nil || m.config.Auth == nil {
		return nil
	}

	if cfg := m.config.Auth.OAuth2; cfg != nil {
		token, err := m.ensureOAuthToken(ctx, cfg)
		if err != nil {
			return err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	}

	if cfg := m.config.Auth.CustomHeader; cfg != nil {
		header := strings.TrimSpace(cfg.Header)
		token := strings.TrimSpace(cfg.Token)
		if header != "" && token != "" {
			req.Header.Set(header, token)
			return nil
		}
	}

	if cfg := m.config.Auth.BearerToken; cfg != nil && strings.TrimSpace(cfg.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
		return nil
	}

	if cfg := m.config.Auth.BasicAuth; cfg != nil {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}

	return nil
}

func (m *HTTPResourceServerManager) ensureOAuthToken(ctx context.Context, cfg *HTTPResourceServerOAuth2Config) (string, error) {
	m.oauthMu.Lock()
	defer m.oauthMu.Unlock()

	if m.oauthToken != nil && time.Until(m.oauthToken.Expiry) > defaultOAuthExpirySkew {
		return m.oauthToken.AccessToken, nil
	}

	token, err := m.fetchOAuthToken(ctx, cfg)
	if err != nil {
		return "", err
	}
	m.oauthToken = token
	return token.AccessToken, nil
}

func (m *HTTPResourceServerManager) fetchOAuthToken(ctx context.Context, cfg *HTTPResourceServerOAuth2Config) (*oauthToken, error) {
	if cfg == nil {
		return nil, errors.New("oauth2 configuration is required")
	}
	if strings.TrimSpace(cfg.TokenURL) == "" {
		return nil, errors.New("oauth2 token_url is required")
	}

	form := url.Values{}
	grantType := strings.TrimSpace(cfg.GrantType)
	if grantType == "" {
		grantType = "client_credentials"
	}
	form.Set("grant_type", grantType)

	if cfg.ClientID != "" {
		form.Set("client_id", cfg.ClientID)
	}
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}

	switch grantType {
	case "password":
		if cfg.Username == "" || cfg.Password == "" {
			return nil, errors.New("oauth2 password grant requires username and password")
		}
		form.Set("username", cfg.Username)
		form.Set("password", cfg.Password)
	case "client_credentials":
	default:
		return nil, fmt.Errorf("unsupported oauth2 grant_type %q", grantType)
	}

	if cfg.Scope != "" {
		form.Set("scope", cfg.Scope)
	}
	if cfg.Audience != "" {
		form.Set("audience", cfg.Audience)
	}

	formData := form.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(formData))
	if err != nil {
		return nil, fmt.Errorf("failed to build oauth2 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	m.recordRequest(req, []byte(formData))
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth2 token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read oauth2 token response: %w", err)
	}

	m.recordResponse(resp.StatusCode, resp.Header.Clone(), body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{
			Method:     http.MethodPost,
			URL:        cfg.TokenURL,
			StatusCode: resp.StatusCode,
			Body:       body,
		}
	}

	var parsed oauthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse oauth2 token response: %w", err)
	}

	if parsed.AccessToken == "" {
		return nil, errors.New("oauth2 token response missing access_token")
	}

	expiry := time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	if parsed.ExpiresIn == 0 {
		expiry = time.Now().Add(5 * time.Minute)
	}

	return &oauthToken{
		AccessToken: parsed.AccessToken,
		TokenType:   parsed.TokenType,
		Expiry:      expiry,
	}, nil
}

func (m *HTTPResourceServerManager) LoadOpenAPISpec(source string) ([]byte, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, errors.New("openapi source is required")
	}

	if isHTTPURL(trimmed) {
		if m.client == nil {
			if err := m.Init(); err != nil {
				return nil, err
			}
		}
		return m.fetchSpecURL(trimmed)
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to read openapi file %q: %w", trimmed, err)
	}
	return data, nil
}

func (m *HTTPResourceServerManager) fetchSpecURL(fullURL string) ([]byte, error) {
	spec := &HTTPRequestSpec{
		Method: http.MethodGet,
		Path:   fullURL,
		Accept: "application/json, application/yaml, application/x-yaml, text/yaml, */*",
	}
	resp, err := m.executeRequest(spec, defaultRequestTimeout, nil, true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func isHTTPURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

type oauthToken struct {
	AccessToken string
	TokenType   string
	Expiry      time.Time
}

type oauthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}
