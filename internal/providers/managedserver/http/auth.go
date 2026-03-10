package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
)

type authMode int

const (
	authModeUnknown authMode = iota
	authModeOAuth2
	authModeBasic
	authModeCustomHeaders
)

type authConfig struct {
	mode          authMode
	oauth2        config.OAuth2
	basicAuth     config.BasicAuth
	customHeaders []config.HeaderTokenAuth
}

func buildAuthConfig(cfg *config.HTTPAuth) (authConfig, error) {
	if cfg == nil {
		return authConfig{}, faults.NewValidationError("managed-server.http.auth is required", nil)
	}

	setCount := 0
	if cfg.OAuth2 != nil {
		setCount++
	}
	if cfg.BasicAuth != nil {
		setCount++
	}
	if len(cfg.CustomHeaders) > 0 {
		setCount++
	}
	if setCount != 1 {
		return authConfig{}, faults.NewValidationError("managed-server.http.auth must define exactly one auth mode", nil)
	}

	switch {
	case cfg.OAuth2 != nil:
		oauth := *cfg.OAuth2
		if strings.TrimSpace(oauth.TokenURL) == "" ||
			strings.TrimSpace(oauth.GrantType) == "" ||
			strings.TrimSpace(oauth.ClientID) == "" ||
			strings.TrimSpace(oauth.ClientSecret) == "" {
			return authConfig{}, faults.NewValidationError("managed-server.http.auth.oauth2 requires token-url, grant-type, client-id, client-secret", nil)
		}
		if strings.TrimSpace(oauth.GrantType) != config.OAuthClientCreds {
			return authConfig{}, faults.NewValidationError("managed-server.http.auth.oauth2.grant-type supports only client_credentials", nil)
		}
		tokenURL, err := url.Parse(oauth.TokenURL)
		if err != nil || tokenURL.Scheme == "" || tokenURL.Host == "" {
			return authConfig{}, faults.NewValidationError("managed-server.http.auth.oauth2.token-url is invalid", err)
		}

		return authConfig{mode: authModeOAuth2, oauth2: oauth}, nil
	case cfg.BasicAuth != nil:
		basic := *cfg.BasicAuth
		if basic.Username == "" || basic.Password == "" {
			return authConfig{}, faults.NewValidationError("managed-server.http.auth.basic-auth requires username and password", nil)
		}
		return authConfig{mode: authModeBasic, basicAuth: basic}, nil
	case len(cfg.CustomHeaders) > 0:
		customHeaders := make([]config.HeaderTokenAuth, 0, len(cfg.CustomHeaders))
		for idx, custom := range cfg.CustomHeaders {
			custom.Header = strings.TrimSpace(custom.Header)
			custom.Prefix = strings.TrimSpace(custom.Prefix)
			custom.Value = strings.TrimSpace(custom.Value)
			if custom.Header == "" || custom.Value == "" {
				return authConfig{}, faults.NewValidationError(
					fmt.Sprintf("managed-server.http.auth.custom-headers[%d] requires header and value", idx),
					nil,
				)
			}
			customHeaders = append(customHeaders, custom)
		}
		return authConfig{mode: authModeCustomHeaders, customHeaders: customHeaders}, nil
	default:
		return authConfig{}, faults.NewValidationError("managed-server.http.auth is invalid", nil)
	}
}

func (g *Client) applyAuth(ctx context.Context, request *http.Request) error {
	switch g.auth.mode {
	case authModeOAuth2:
		debugctx.Detailf(ctx, "auth mode=oauth2 token_url=%q client_id=%q", g.auth.oauth2.TokenURL, g.auth.oauth2.ClientID)
		if debugctx.Insecure(ctx) {
			debugctx.Detailf(ctx, "auth oauth2 client_secret=%q", g.auth.oauth2.ClientSecret)
		}
		token, err := g.oauthToken(ctx)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		if debugctx.Insecure(ctx) {
			debugctx.Detailf(ctx, "auth oauth2 access_token=%q", token)
		}
	case authModeBasic:
		debugctx.Detailf(ctx, "auth mode=basic username=%q", g.auth.basicAuth.Username)
		if debugctx.Insecure(ctx) {
			debugctx.Detailf(ctx, "auth basic password=%q", g.auth.basicAuth.Password)
		}
		request.SetBasicAuth(g.auth.basicAuth.Username, g.auth.basicAuth.Password)
	case authModeCustomHeaders:
		debugctx.Detailf(ctx, "auth mode=custom-headers count=%d", len(g.auth.customHeaders))
		for _, customHeader := range g.auth.customHeaders {
			value := customHeader.Value
			if customHeader.Prefix != "" {
				value = customHeader.Prefix + " " + value
			}
			if debugctx.Insecure(ctx) {
				debugctx.Detailf(ctx, "auth custom-header %s: %s", customHeader.Header, value)
			}
			request.Header.Set(customHeader.Header, value)
		}
	default:
		return faults.NewValidationError("managed-server.http.auth mode is not configured", nil)
	}
	return nil
}

func (g *Client) oauthToken(ctx context.Context) (string, error) {
	g.oauthMu.Lock()
	defer g.oauthMu.Unlock()

	if g.oauthAccessToken != "" && time.Now().Before(g.oauthExpiresAt.Add(-30*time.Second)) {
		debugctx.Detailf(ctx, "oauth2 using cached token expires_at=%s", g.oauthExpiresAt.Format(time.RFC3339))
		return g.oauthAccessToken, nil
	}

	debugctx.Detailf(ctx, "oauth2 requesting new token token_url=%q grant_type=%q", g.auth.oauth2.TokenURL, g.auth.oauth2.GrantType)

	formValues := url.Values{}
	formValues.Set("grant_type", g.auth.oauth2.GrantType)
	formValues.Set("client_id", g.auth.oauth2.ClientID)
	formValues.Set("client_secret", g.auth.oauth2.ClientSecret)
	if strings.TrimSpace(g.auth.oauth2.Scope) != "" {
		formValues.Set("scope", g.auth.oauth2.Scope)
	}
	if strings.TrimSpace(g.auth.oauth2.Audience) != "" {
		formValues.Set("audience", g.auth.oauth2.Audience)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		g.auth.oauth2.TokenURL,
		strings.NewReader(formValues.Encode()),
	)
	if err != nil {
		return "", internalError("failed to create oauth2 token request", err)
	}
	request.Header.Set("Accept", defaultMediaType)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := g.doRequest(ctx, "oauth2-token", request)
	if err != nil {
		return "", transportError("oauth2 token request failed", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", transportError("failed to read oauth2 token response", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		debugctx.Infof(
			ctx,
			"oauth2 token error status=%d response_body=%s",
			response.StatusCode,
			summarizeBodyForLevel(body, debugctx.Level(ctx)),
		)
		return "", authError(
			fmt.Sprintf("oauth2 token request failed with status %d: %s", response.StatusCode, summarizeBody(body)),
			nil,
		)
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", authError("oauth2 token response is not valid JSON", err)
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return "", authError("oauth2 token response does not include access_token", nil)
	}

	expiresAt := time.Now().Add(time.Hour)
	if tokenResponse.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	}

	g.oauthAccessToken = tokenResponse.AccessToken
	g.oauthExpiresAt = expiresAt

	debugctx.Detailf(ctx, "oauth2 token acquired expires_in=%ds expires_at=%s", tokenResponse.ExpiresIn, expiresAt.Format(time.RFC3339))

	return g.oauthAccessToken, nil
}
