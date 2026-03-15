package http

import (
	"context"
	"crypto/tls"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	"github.com/crmarques/declarest/internal/providers/tlsconfig"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultMediaType   = "application/json"
)

var _ managedserver.ManagedServerClient = (*Client)(nil)
var _ managedserver.AccessTokenProvider = (*Client)(nil)

type Client struct {
	baseURL          *url.URL
	defaultHeaders   map[string]string
	auth             authConfig
	client           *http.Client
	throttle         *requestThrottleGate
	tlsDebug         tlsDebugInfo
	openAPISource    string
	metadataRenderer metadata.ResourceOperationSpecRenderer

	openapiMu     sync.Mutex
	openapiLoaded bool
	openapiDoc    map[string]any

	oauthMu          sync.Mutex
	oauthAccessToken string
	oauthExpiresAt   time.Time

	promptRuntime *promptauth.Runtime
}

type ClientOption func(*Client)

func WithMetadataRenderer(renderer metadata.ResourceOperationSpecRenderer) ClientOption {
	return func(g *Client) {
		if g == nil {
			return
		}
		g.metadataRenderer = renderer
	}
}

func WithPromptRuntime(runtime *promptauth.Runtime) ClientOption {
	return func(g *Client) {
		if g == nil {
			return
		}
		g.promptRuntime = runtime
	}
}

func NewClient(cfg config.HTTPServer, opts ...ClientOption) (*Client, error) {
	baseURL, err := parseBaseURL(cfg.BaseURL)
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
	transport.Proxy = nil

	client := &Client{
		baseURL:        baseURL,
		defaultHeaders: maps.Clone(cfg.DefaultHeaders),
		client: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: transport,
		},
		tlsDebug:      newTLSDebugInfo(cfg.TLS),
		openAPISource: strings.TrimSpace(cfg.OpenAPI),
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
	auth, err := buildAuthConfig(cfg.Auth, client.promptRuntime)
	if err != nil {
		return nil, err
	}
	client.auth = auth
	if proxyFunc, err := buildProxyFunc(cfg.Proxy, client.promptRuntime); err != nil {
		return nil, err
	} else if proxyFunc != nil {
		transport.Proxy = proxyFunc
	}
	return client, nil
}

func (g *Client) Get(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) (resource.Content, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resolvedResource, md, metadata.OperationGet)
	if err != nil {
		return resource.Content{}, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return resource.Content{}, err
	}

	content, err := decodeResponseBody(body, headers, g.requestBodyDescriptor(resolvedResource, md))
	if err != nil {
		return resource.Content{}, err
	}

	value, err := g.applyOperationPayloadTransforms(ctx, content.Value, spec)
	if err != nil {
		return resource.Content{}, err
	}
	content.Value = value
	return content, nil
}

func (g *Client) Create(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) (resource.Content, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resolvedResource, md, metadata.OperationCreate)
	if err != nil {
		return resource.Content{}, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return resource.Content{}, err
	}

	return decodeResponseBody(body, headers, g.requestBodyDescriptor(resolvedResource, md))
}

func (g *Client) Update(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) (resource.Content, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resolvedResource, md, metadata.OperationUpdate)
	if err != nil {
		return resource.Content{}, err
	}

	body, headers, err := g.execute(ctx, spec)
	if err != nil {
		return resource.Content{}, err
	}

	return decodeResponseBody(body, headers, g.requestBodyDescriptor(resolvedResource, md))
}

func (g *Client) Delete(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) error {
	spec, err := g.BuildRequestFromMetadata(ctx, resolvedResource, md, metadata.OperationDelete)
	if err != nil {
		return err
	}

	_, _, err = g.execute(ctx, spec)
	return err
}

func (g *Client) List(ctx context.Context, collectionPath string, md metadata.ResourceMetadata) ([]resource.Resource, error) {
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

func (g *Client) Exists(ctx context.Context, resolvedResource resource.Resource, md metadata.ResourceMetadata) (bool, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resolvedResource, md, metadata.OperationGet)
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

func (g *Client) GetAccessToken(ctx context.Context) (string, error) {
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
