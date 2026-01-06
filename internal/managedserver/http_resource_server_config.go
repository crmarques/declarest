package managedserver

type HTTPResourceServerConfig struct {
	BaseURL        string                        `mapstructure:"base_url" yaml:"base_url,omitempty" json:"base_url,omitempty"`
	OpenAPI        string                        `mapstructure:"openapi" yaml:"openapi,omitempty" json:"openapi,omitempty"`
	DefaultHeaders map[string]string             `mapstructure:"default_headers" yaml:"default_headers,omitempty" json:"default_headers,omitempty"`
	Auth           *HTTPResourceServerAuthConfig `mapstructure:"auth" yaml:"auth,omitempty" json:"auth,omitempty"`
	TLS            *HTTPResourceServerTLSConfig  `mapstructure:"tls" yaml:"tls,omitempty" json:"tls,omitempty"`
}

type HTTPResourceServerAuthConfig struct {
	OAuth2       *HTTPResourceServerOAuth2Config       `mapstructure:"oauth2" yaml:"oauth2,omitempty" json:"oauth2,omitempty"`
	CustomHeader *HTTPResourceServerCustomHeaderConfig `mapstructure:"custom_header" yaml:"custom_header,omitempty" json:"custom_header,omitempty"`
	BasicAuth    *HTTPResourceServerBasicAuthConfig    `mapstructure:"basic_auth" yaml:"basic_auth,omitempty" json:"basic_auth,omitempty"`
	BearerToken  *HTTPResourceServerBearerTokenConfig  `mapstructure:"bearer_token" yaml:"bearer_token,omitempty" json:"bearer_token,omitempty"`
}

type HTTPResourceServerOAuth2Config struct {
	TokenURL     string `mapstructure:"token_url" yaml:"token_url,omitempty" json:"token_url,omitempty"`
	GrantType    string `mapstructure:"grant_type" yaml:"grant_type,omitempty" json:"grant_type,omitempty"`
	ClientID     string `mapstructure:"client_id" yaml:"client_id,omitempty" json:"client_id,omitempty"`
	ClientSecret string `mapstructure:"client_secret" yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
	Username     string `mapstructure:"username" yaml:"username,omitempty" json:"username,omitempty"`
	Password     string `mapstructure:"password" yaml:"password,omitempty" json:"password,omitempty"`
	Scope        string `mapstructure:"scope" yaml:"scope,omitempty" json:"scope,omitempty"`
	Audience     string `mapstructure:"audience" yaml:"audience,omitempty" json:"audience,omitempty"`
}

type HTTPResourceServerBasicAuthConfig struct {
	Username string `mapstructure:"username" yaml:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password" yaml:"password,omitempty" json:"password,omitempty"`
}

type HTTPResourceServerBearerTokenConfig struct {
	Token string `mapstructure:"token" yaml:"token,omitempty" json:"token,omitempty"`
}

type HTTPResourceServerCustomHeaderConfig struct {
	Header string `mapstructure:"header" yaml:"header,omitempty" json:"header,omitempty"`
	Token  string `mapstructure:"token" yaml:"token,omitempty" json:"token,omitempty"`
}

type HTTPResourceServerTLSConfig struct {
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
}
