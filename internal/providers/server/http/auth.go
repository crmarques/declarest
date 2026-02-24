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
)

type authMode int

const (
	authModeUnknown authMode = iota
	authModeOAuth2
	authModeBasic
	authModeBearer
	authModeCustomHeader
)

type authConfig struct {
	mode         authMode
	oauth2       config.OAuth2
	basicAuth    config.BasicAuth
	bearerToken  config.BearerTokenAuth
	customHeader config.HeaderTokenAuth
}

func buildAuthConfig(cfg *config.HTTPAuth) (authConfig, error) {
	if cfg == nil {
		return authConfig{}, validationError("resource-server.http.auth is required", nil)
	}

	setCount := 0
	if cfg.OAuth2 != nil {
		setCount++
	}
	if cfg.BasicAuth != nil {
		setCount++
	}
	if cfg.BearerToken != nil {
		setCount++
	}
	if cfg.CustomHeader != nil {
		setCount++
	}
	if setCount != 1 {
		return authConfig{}, validationError("resource-server.http.auth must define exactly one auth mode", nil)
	}

	switch {
	case cfg.OAuth2 != nil:
		oauth := *cfg.OAuth2
		if strings.TrimSpace(oauth.TokenURL) == "" ||
			strings.TrimSpace(oauth.GrantType) == "" ||
			strings.TrimSpace(oauth.ClientID) == "" ||
			strings.TrimSpace(oauth.ClientSecret) == "" {
			return authConfig{}, validationError("resource-server.http.auth.oauth2 requires token-url, grant-type, client-id, client-secret", nil)
		}
		if strings.TrimSpace(oauth.GrantType) != config.OAuthClientCreds {
			return authConfig{}, validationError("resource-server.http.auth.oauth2.grant-type supports only client_credentials", nil)
		}
		tokenURL, err := url.Parse(oauth.TokenURL)
		if err != nil || tokenURL.Scheme == "" || tokenURL.Host == "" {
			return authConfig{}, validationError("resource-server.http.auth.oauth2.token-url is invalid", err)
		}

		return authConfig{mode: authModeOAuth2, oauth2: oauth}, nil
	case cfg.BasicAuth != nil:
		basic := *cfg.BasicAuth
		if basic.Username == "" || basic.Password == "" {
			return authConfig{}, validationError("resource-server.http.auth.basic-auth requires username and password", nil)
		}
		return authConfig{mode: authModeBasic, basicAuth: basic}, nil
	case cfg.BearerToken != nil:
		bearer := *cfg.BearerToken
		if bearer.Token == "" {
			return authConfig{}, validationError("resource-server.http.auth.bearer-token.token is required", nil)
		}
		return authConfig{mode: authModeBearer, bearerToken: bearer}, nil
	case cfg.CustomHeader != nil:
		custom := *cfg.CustomHeader
		if custom.Header == "" || custom.Token == "" {
			return authConfig{}, validationError("resource-server.http.auth.custom-header requires header and token", nil)
		}
		return authConfig{mode: authModeCustomHeader, customHeader: custom}, nil
	default:
		return authConfig{}, validationError("resource-server.http.auth is invalid", nil)
	}
}

func (g *HTTPResourceServerGateway) applyAuth(ctx context.Context, request *http.Request) error {
	switch g.auth.mode {
	case authModeOAuth2:
		token, err := g.oauthToken(ctx)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
	case authModeBasic:
		request.SetBasicAuth(g.auth.basicAuth.Username, g.auth.basicAuth.Password)
	case authModeBearer:
		request.Header.Set("Authorization", "Bearer "+g.auth.bearerToken.Token)
	case authModeCustomHeader:
		request.Header.Set(g.auth.customHeader.Header, g.auth.customHeader.Token)
	default:
		return validationError("resource-server.http.auth mode is not configured", nil)
	}
	return nil
}

func (g *HTTPResourceServerGateway) oauthToken(ctx context.Context) (string, error) {
	g.oauthMu.Lock()
	if g.oauthAccessToken != "" && time.Now().Before(g.oauthExpiresAt.Add(-30*time.Second)) {
		token := g.oauthAccessToken
		g.oauthMu.Unlock()
		return token, nil
	}
	g.oauthMu.Unlock()

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
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", transportError("failed to read oauth2 token response", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
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

	g.oauthMu.Lock()
	g.oauthAccessToken = tokenResponse.AccessToken
	g.oauthExpiresAt = expiresAt
	g.oauthMu.Unlock()

	return tokenResponse.AccessToken, nil
}
