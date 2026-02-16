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
	"github.com/crmarques/declarest/internal/support/tlsconfig"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/server"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultMediaType   = "application/json"
)

var _ server.ResourceServer = (*HTTPResourceServerGateway)(nil)

type HTTPResourceServerGateway struct {
	baseURL        *url.URL
	defaultHeaders map[string]string
	auth           authConfig
	client         *http.Client
	openAPISource  string

	openapiMu     sync.Mutex
	openapiLoaded bool
	openapiDoc    map[string]any
	openapiErr    error

	oauthMu          sync.Mutex
	oauthAccessToken string
	oauthExpiresAt   time.Time
}

func NewHTTPResourceServerGateway(cfg config.HTTPServer) (*HTTPResourceServerGateway, error) {
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

	return &HTTPResourceServerGateway{
		baseURL:        baseURL,
		defaultHeaders: cloneStringMap(cfg.DefaultHeaders),
		auth:           auth,
		client: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: transport,
		},
		openAPISource: strings.TrimSpace(cfg.OpenAPI),
	}, nil
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

	return decodeJSONResponse(body)
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

	return g.decodeListResponse(collectionPath, md, body)
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

func parseBaseURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, validationError("managed-server.http.base-url is required", nil)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, validationError("managed-server.http.base-url is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, validationError("managed-server.http.base-url must use http or https", nil)
	}
	if parsed.Host == "" {
		return nil, validationError("managed-server.http.base-url host is required", nil)
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
