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
	"github.com/crmarques/declarest/internal/providers/shared/tlsconfig"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultMediaType   = "application/json"
)

var _ server.ResourceServer = (*HTTPResourceServerGateway)(nil)
var _ server.AccessTokenProvider = (*HTTPResourceServerGateway)(nil)

type HTTPResourceServerGateway struct {
	baseURL          *url.URL
	defaultHeaders   map[string]string
	auth             authConfig
	client           *http.Client
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

type GatewayOption func(*HTTPResourceServerGateway)

func WithMetadataRenderer(renderer metadata.ResourceOperationSpecRenderer) GatewayOption {
	return func(g *HTTPResourceServerGateway) {
		if g == nil {
			return
		}
		g.metadataRenderer = renderer
	}
}

func WithResourceFormat(format string) GatewayOption {
	return func(g *HTTPResourceServerGateway) {
		if g == nil {
			return
		}
		g.resourceFormat = metadata.NormalizeResourceFormat(format)
	}
}

func NewHTTPResourceServerGateway(cfg config.HTTPServer, opts ...GatewayOption) (*HTTPResourceServerGateway, error) {
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

	gateway := &HTTPResourceServerGateway{
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
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(gateway)
	}
	return gateway, nil
}

func (g *HTTPResourceServerGateway) SetResourceFormat(format string) {
	if g == nil {
		return
	}
	g.resourceFormat = metadata.NormalizeResourceFormat(format)
}

func (g *HTTPResourceServerGateway) Get(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, metadata.OperationGet)
	if err != nil {
		return nil, err
	}

	body, _, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	value, err := decodeJSONResponse(body)
	if err != nil {
		return nil, err
	}

	return g.applyOperationPayloadTransforms(ctx, value, spec)
}

func (g *HTTPResourceServerGateway) Create(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, metadata.OperationCreate)
	if err != nil {
		return nil, err
	}

	body, _, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return decodeJSONResponse(body)
}

func (g *HTTPResourceServerGateway) Update(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, metadata.OperationUpdate)
	if err != nil {
		return nil, err
	}

	body, _, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return decodeJSONResponse(body)
}

func (g *HTTPResourceServerGateway) Delete(ctx context.Context, resourceInfo resource.Resource) error {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, metadata.OperationDelete)
	if err != nil {
		return err
	}

	_, _, err = g.execute(ctx, spec)
	return err
}

func (g *HTTPResourceServerGateway) List(ctx context.Context, collectionPath string, md metadata.ResourceMetadata) ([]resource.Resource, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resource.Resource{
		LogicalPath:    collectionPath,
		CollectionPath: collectionPath,
		Metadata:       md,
	}, metadata.OperationList)
	if err != nil {
		return nil, err
	}

	body, _, err := g.execute(ctx, spec)
	if err != nil {
		return nil, err
	}

	return g.decodeListResponse(ctx, collectionPath, md, spec, body)
}

func (g *HTTPResourceServerGateway) Exists(ctx context.Context, resourceInfo resource.Resource) (bool, error) {
	spec, err := g.BuildRequestFromMetadata(ctx, resourceInfo, metadata.OperationGet)
	if err != nil {
		return false, err
	}

	_, _, err = g.execute(ctx, spec)
	if err == nil {
		return true, nil
	}
	if isTypedCategory(err, NotFoundError) {
		return false, nil
	}
	return false, err
}

func (g *HTTPResourceServerGateway) GetAccessToken(ctx context.Context) (string, error) {
	if g == nil {
		return "", validationError("resource server is not configured", nil)
	}
	if g.auth.mode != authModeOAuth2 {
		return "", validationError("resource-server.http.auth.oauth2 is not configured", nil)
	}
	return g.oauthToken(ctx)
}

func parseBaseURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, validationError("resource-server.http.base-url is required", nil)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, validationError("resource-server.http.base-url is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, validationError("resource-server.http.base-url must use http or https", nil)
	}
	if parsed.Host == "" {
		return nil, validationError("resource-server.http.base-url host is required", nil)
	}

	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed, nil
}

func buildTLSConfig(tlsSettings *config.TLS) (*tls.Config, error) {
	return tlsconfig.BuildTLSConfig(tlsSettings, "resource-server.http")
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
