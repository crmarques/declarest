package http

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/providers/tlsconfig"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultMediaType   = "application/json"
)

var _ managedserver.ManagedServerClient = (*HTTPManagedServerClient)(nil)
var _ managedserver.AccessTokenProvider = (*HTTPManagedServerClient)(nil)

type HTTPManagedServerClient struct {
	baseURL          *url.URL
	defaultHeaders   map[string]string
	auth             authConfig
	client           *http.Client
	throttle         *requestThrottleGate
	resourceFormat   string
	tlsDebug         tlsDebugInfo
	openAPISource    string
	metadataRenderer metadata.ResourceOperationSpecRenderer

	openapiMu     sync.Mutex
	openapiLoaded bool
	openapiDoc    map[string]any
	openapiErr    error

	oauthMu          sync.Mutex
	oauthAccessToken string
	oauthExpiresAt   time.Time
}

type ManagedServerClientOption func(*HTTPManagedServerClient)

func WithMetadataRenderer(renderer metadata.ResourceOperationSpecRenderer) ManagedServerClientOption {
	return func(g *HTTPManagedServerClient) {
		if g == nil {
			return
		}
		g.metadataRenderer = renderer
	}
}

func WithResourceFormat(format string) ManagedServerClientOption {
	return func(g *HTTPManagedServerClient) {
		if g == nil {
			return
		}
		g.resourceFormat = metadata.NormalizeResourceFormat(format)
	}
}

func NewHTTPManagedServerClient(cfg config.HTTPServer, opts ...ManagedServerClientOption) (*HTTPManagedServerClient, error) {
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	auth, err := buildAuthConfig(cfg.Auth)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	if err := validateOpenAPISource(cfg.OpenAPI); err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	if proxyFunc, err := buildProxyFunc(cfg.Proxy); err != nil {
		return nil, err
	} else if proxyFunc != nil {
		transport.Proxy = proxyFunc
	}

	client := &HTTPManagedServerClient{
		baseURL:        baseURL,
		defaultHeaders: cloneStringMap(cfg.DefaultHeaders),
		auth:           auth,
		client: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: transport,
		},
		resourceFormat: config.ResourceFormatJSON,
		tlsDebug:       newTLSDebugInfo(cfg.TLS),
		openAPISource:  strings.TrimSpace(cfg.OpenAPI),
	}
	throttle, err := buildRequestThrottle(cfg.RequestThrottling)
	if err != nil {
		return nil, err
	}
	client.throttle = throttle
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(client)
	}
	return client, nil
}

func (g *HTTPManagedServerClient) SetResourceFormat(format string) {
	if g == nil {
		return
	}
	g.resourceFormat = metadata.NormalizeResourceFormat(format)
}

func (g *HTTPManagedServerClient) Get(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, md, metadata.OperationGet)
	if err != nil {
		return nil, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	value, err := decodeResponseBody(body, headers, g.metadataPayloadType(md))
	if err != nil {
		return nil, err
	}

	return g.applyOperationPayloadTransforms(ctx, value, spec)
}

func (g *HTTPManagedServerClient) Create(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, md, metadata.OperationCreate)
	if err != nil {
		return nil, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return decodeResponseBody(body, headers, g.metadataPayloadType(md))
}

func (g *HTTPManagedServerClient) Update(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, md, metadata.OperationUpdate)
	if err != nil {
		return nil, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return decodeResponseBody(body, headers, g.metadataPayloadType(md))
}

func (g *HTTPManagedServerClient) Delete(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) error {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, md, metadata.OperationDelete)
	if err != nil {
		return err
	}

	_, _, err = g.execute(ctx, spec)
	return err
}

func (g *HTTPManagedServerClient) List(ctx context.Context, collectionPath string, md metadata.ResourceMetadata) ([]resource.Resource, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resource.Resource{
		LogicalPath:    collectionPath,
		CollectionPath: collectionPath,
	}, md, metadata.OperationList)
	if err != nil {
		return nil, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return g.decodeListResponse(ctx, collectionPath, md, spec, body, headers)
}

func (g *HTTPManagedServerClient) Exists(ctx context.Context, resourceInfo resource.Resource, md metadata.ResourceMetadata) (bool, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, md, metadata.OperationGet)
	if err != nil {
		return false, err
	}

	_, _, err = g.execute(ctx, spec)
	if err == nil {
		return true, nil
	}
	if faults.IsCategory(err, NotFoundError) {
		return false, nil
	}
	return false, err
}

func (g *HTTPManagedServerClient) GetAccessToken(ctx context.Context) (string, error) {
	if g == nil {
		return "", faults.NewValidationError("managed server is not configured", nil)
	}
	if g.auth.mode != authModeOAuth2 {
		return "", faults.NewValidationError("managed-server.http.auth.oauth2 is not configured", nil)
	}
	return g.oauthToken(ctx)
}

func parseBaseURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, faults.NewValidationError("managed-server.http.base-url is required", nil)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, faults.NewValidationError("managed-server.http.base-url is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, faults.NewValidationError("managed-server.http.base-url must use http or https", nil)
	}
	if parsed.Host == "" {
		return nil, faults.NewValidationError("managed-server.http.base-url host is required", nil)
	}

	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed, nil
}

func buildTLSConfig(tlsSettings *config.TLS) (*tls.Config, error) {
	return tlsconfig.BuildTLSConfig(tlsSettings, "managed-server.http")
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
