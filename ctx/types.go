package ctx

import (
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

type Runtime struct {
	Name        string
	Environment map[string]string
	Repository  repository.Manager
	Metadata    metadata.Manager
	Server      server.Manager
	Secrets     secrets.Manager
}

const (
	ContextFileEnvVar  = "DECLAREST_CONTEXT_FILE"
	DefaultCatalogPath = "~/declarest/config/contexts.yaml"
	ResourceFormatJSON = "json"
	ResourceFormatYAML = "yaml"
	GitProviderGitHub  = "github"
	OAuthClientCreds   = "client_credentials"
)

type Catalog struct {
	Contexts   []Config `yaml:"contexts"`
	CurrentCtx string   `yaml:"current-ctx"`
}

type Config struct {
	Name          string               `yaml:"name"`
	Repository    RepositoryConfig     `yaml:"repository"`
	ManagedServer *ManagedServerConfig `yaml:"managed-server,omitempty"`
	SecretStore   *SecretStoreConfig   `yaml:"secret-store,omitempty"`
	Metadata      MetadataConfig       `yaml:"metadata,omitempty"`
	Preferences   map[string]string    `yaml:"preferences,omitempty"`
}

type RepositoryConfig struct {
	ResourceFormat string                      `yaml:"resource-format,omitempty"`
	Git            *GitRepositoryConfig        `yaml:"git,omitempty"`
	Filesystem     *FilesystemRepositoryConfig `yaml:"filesystem,omitempty"`
}

type GitRepositoryConfig struct {
	Local  GitLocalConfig   `yaml:"local"`
	Remote *GitRemoteConfig `yaml:"remote,omitempty"`
}

type GitLocalConfig struct {
	BaseDir string `yaml:"base-dir"`
}

type GitRemoteConfig struct {
	URL      string         `yaml:"url"`
	Branch   string         `yaml:"branch,omitempty"`
	Provider string         `yaml:"provider,omitempty"`
	AutoSync bool           `yaml:"auto-sync,omitempty"`
	Auth     *GitAuthConfig `yaml:"auth,omitempty"`
	TLS      *TLSConfig     `yaml:"tls,omitempty"`
}

type GitAuthConfig struct {
	BasicAuth *BasicAuthConfig `yaml:"basic-auth,omitempty"`
	SSH       *SSHAuthConfig   `yaml:"ssh,omitempty"`
	AccessKey *AccessKeyConfig `yaml:"access-key,omitempty"`
}

type FilesystemRepositoryConfig struct {
	BaseDir string `yaml:"base-dir"`
}

type ManagedServerConfig struct {
	HTTP *HTTPServerConfig `yaml:"http,omitempty"`
}

type HTTPServerConfig struct {
	BaseURL        string            `yaml:"base-url"`
	OpenAPI        string            `yaml:"openapi,omitempty"`
	DefaultHeaders map[string]string `yaml:"default-headers,omitempty"`
	Auth           *HTTPAuthConfig   `yaml:"auth,omitempty"`
	TLS            *TLSConfig        `yaml:"tls,omitempty"`
}

type HTTPAuthConfig struct {
	OAuth2       *OAuth2Config           `yaml:"oauth2,omitempty"`
	BasicAuth    *BasicAuthConfig        `yaml:"basic-auth,omitempty"`
	BearerToken  *BearerTokenConfig      `yaml:"bearer-token,omitempty"`
	CustomHeader *CustomHeaderAuthConfig `yaml:"custom-header,omitempty"`
}

type OAuth2Config struct {
	TokenURL     string `yaml:"token-url"`
	GrantType    string `yaml:"grant-type"`
	ClientID     string `yaml:"client-id"`
	ClientSecret string `yaml:"client-secret"`
	Username     string `yaml:"username,omitempty"`
	Password     string `yaml:"password,omitempty"`
	Scope        string `yaml:"scope,omitempty"`
	Audience     string `yaml:"audience,omitempty"`
}

type BasicAuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type BearerTokenConfig struct {
	Token string `yaml:"token"`
}

type CustomHeaderAuthConfig struct {
	Header string `yaml:"header"`
	Token  string `yaml:"token"`
}

type SSHAuthConfig struct {
	User                  string `yaml:"user"`
	PrivateKeyFile        string `yaml:"private-key-file"`
	Passphrase            string `yaml:"passphrase,omitempty"`
	KnownHostsFile        string `yaml:"known-hosts-file,omitempty"`
	InsecureIgnoreHostKey bool   `yaml:"insecure-ignore-host-key,omitempty"`
}

type AccessKeyConfig struct {
	Token string `yaml:"token"`
}

type SecretStoreConfig struct {
	File  *FileSecretStoreConfig  `yaml:"file,omitempty"`
	Vault *VaultSecretStoreConfig `yaml:"vault,omitempty"`
}

type FileSecretStoreConfig struct {
	Path           string     `yaml:"path"`
	Key            string     `yaml:"key,omitempty"`
	KeyFile        string     `yaml:"key-file,omitempty"`
	Passphrase     string     `yaml:"passphrase,omitempty"`
	PassphraseFile string     `yaml:"passphrase-file,omitempty"`
	KDF            *KDFConfig `yaml:"kdf,omitempty"`
}

type KDFConfig struct {
	Time    int `yaml:"time,omitempty"`
	Memory  int `yaml:"memory,omitempty"`
	Threads int `yaml:"threads,omitempty"`
}

type VaultSecretStoreConfig struct {
	Address    string           `yaml:"address"`
	Mount      string           `yaml:"mount,omitempty"`
	PathPrefix string           `yaml:"path-prefix,omitempty"`
	KVVersion  int              `yaml:"kv-version,omitempty"`
	Auth       *VaultAuthConfig `yaml:"auth,omitempty"`
	TLS        *TLSConfig       `yaml:"tls,omitempty"`
}

type VaultAuthConfig struct {
	Token    string             `yaml:"token,omitempty"`
	Password *VaultPasswordAuth `yaml:"password,omitempty"`
	AppRole  *VaultAppRoleAuth  `yaml:"approle,omitempty"`
}

type VaultPasswordAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Mount    string `yaml:"mount,omitempty"`
}

type VaultAppRoleAuth struct {
	RoleID   string `yaml:"role-id"`
	SecretID string `yaml:"secret-id"`
	Mount    string `yaml:"mount,omitempty"`
}

type TLSConfig struct {
	CACertFile         string `yaml:"ca-cert-file,omitempty"`
	ClientCertFile     string `yaml:"client-cert-file,omitempty"`
	ClientKeyFile      string `yaml:"client-key-file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure-skip-verify,omitempty"`
}

type MetadataConfig struct {
	BaseDir string `yaml:"base-dir,omitempty"`
}
