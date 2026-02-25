package config

type ContextSelection struct {
	Name      string
	Overrides map[string]string
}

const (
	ContextFileEnvVar         = "DECLAREST_CONTEXTS_FILE"
	DefaultContextCatalogPath = "~/.declarest/configs/contexts.yaml"
	ResourceFormatJSON        = "json"
	ResourceFormatYAML        = "yaml"
	GitProviderGitHub         = "github"
	OAuthClientCreds          = "client_credentials"
)

type ContextCatalog struct {
	Contexts      []Context `yaml:"contexts"`
	CurrentCtx    string    `yaml:"current-ctx"`
	DefaultEditor string    `yaml:"default-editor,omitempty"`
}

type Context struct {
	Name           string            `yaml:"name"`
	Repository     Repository        `yaml:"repository"`
	ResourceServer *ResourceServer   `yaml:"resource-server,omitempty"`
	SecretStore    *SecretStore      `yaml:"secret-store,omitempty"`
	Metadata       Metadata          `yaml:"metadata,omitempty"`
	Preferences    map[string]string `yaml:"preferences,omitempty"`
}

type Repository struct {
	ResourceFormat string                `yaml:"resource-format,omitempty"`
	Git            *GitRepository        `yaml:"git,omitempty"`
	Filesystem     *FilesystemRepository `yaml:"filesystem,omitempty"`
}

type GitRepository struct {
	Local  GitLocal   `yaml:"local"`
	Remote *GitRemote `yaml:"remote,omitempty"`
}

type GitLocal struct {
	BaseDir  string `yaml:"base-dir"`
	AutoInit *bool  `yaml:"auto-init,omitempty"`
}

func (g GitLocal) AutoInitEnabled() bool {
	if g.AutoInit == nil {
		return true
	}
	return *g.AutoInit
}

type GitRemote struct {
	URL      string   `yaml:"url"`
	Branch   string   `yaml:"branch,omitempty"`
	Provider string   `yaml:"provider,omitempty"`
	AutoSync bool     `yaml:"auto-sync,omitempty"`
	Auth     *GitAuth `yaml:"auth,omitempty"`
	TLS      *TLS     `yaml:"tls,omitempty"`
}

type GitAuth struct {
	BasicAuth *BasicAuth     `yaml:"basic-auth,omitempty"`
	SSH       *SSHAuth       `yaml:"ssh,omitempty"`
	AccessKey *AccessKeyAuth `yaml:"access-key,omitempty"`
}

type FilesystemRepository struct {
	BaseDir string `yaml:"base-dir"`
}

type ResourceServer struct {
	HTTP *HTTPServer `yaml:"http,omitempty"`
}

type HTTPServer struct {
	BaseURL        string            `yaml:"base-url"`
	OpenAPI        string            `yaml:"openapi,omitempty"`
	DefaultHeaders map[string]string `yaml:"default-headers,omitempty"`
	Auth           *HTTPAuth         `yaml:"auth,omitempty"`
	TLS            *TLS              `yaml:"tls,omitempty"`
}

type HTTPAuth struct {
	OAuth2       *OAuth2          `yaml:"oauth2,omitempty"`
	BasicAuth    *BasicAuth       `yaml:"basic-auth,omitempty"`
	BearerToken  *BearerTokenAuth `yaml:"bearer-token,omitempty"`
	CustomHeader *HeaderTokenAuth `yaml:"custom-header,omitempty"`
}

type OAuth2 struct {
	TokenURL     string `yaml:"token-url"`
	GrantType    string `yaml:"grant-type"`
	ClientID     string `yaml:"client-id"`
	ClientSecret string `yaml:"client-secret"`
	Username     string `yaml:"username,omitempty"`
	Password     string `yaml:"password,omitempty"`
	Scope        string `yaml:"scope,omitempty"`
	Audience     string `yaml:"audience,omitempty"`
}

type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type BearerTokenAuth struct {
	Token string `yaml:"token"`
}

type HeaderTokenAuth struct {
	Header string `yaml:"header"`
	Token  string `yaml:"token"`
}

type SSHAuth struct {
	User                  string `yaml:"user"`
	PrivateKeyFile        string `yaml:"private-key-file"`
	Passphrase            string `yaml:"passphrase,omitempty"`
	KnownHostsFile        string `yaml:"known-hosts-file,omitempty"`
	InsecureIgnoreHostKey bool   `yaml:"insecure-ignore-host-key,omitempty"`
}

type AccessKeyAuth struct {
	Token string `yaml:"token"`
}

type SecretStore struct {
	File  *FileSecretStore  `yaml:"file,omitempty"`
	Vault *VaultSecretStore `yaml:"vault,omitempty"`
}

type FileSecretStore struct {
	Path           string `yaml:"path"`
	Key            string `yaml:"key,omitempty"`
	KeyFile        string `yaml:"key-file,omitempty"`
	Passphrase     string `yaml:"passphrase,omitempty"`
	PassphraseFile string `yaml:"passphrase-file,omitempty"`
	KDF            *KDF   `yaml:"kdf,omitempty"`
}

type KDF struct {
	Time    int `yaml:"time,omitempty"`
	Memory  int `yaml:"memory,omitempty"`
	Threads int `yaml:"threads,omitempty"`
}

type VaultSecretStore struct {
	Address    string     `yaml:"address"`
	Mount      string     `yaml:"mount,omitempty"`
	PathPrefix string     `yaml:"path-prefix,omitempty"`
	KVVersion  int        `yaml:"kv-version,omitempty"`
	Auth       *VaultAuth `yaml:"auth,omitempty"`
	TLS        *TLS       `yaml:"tls,omitempty"`
}

type VaultAuth struct {
	Token    string                 `yaml:"token,omitempty"`
	Password *VaultUserPasswordAuth `yaml:"password,omitempty"`
	AppRole  *VaultAppRoleAuth      `yaml:"approle,omitempty"`
}

type VaultUserPasswordAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Mount    string `yaml:"mount,omitempty"`
}

type VaultAppRoleAuth struct {
	RoleID   string `yaml:"role-id"`
	SecretID string `yaml:"secret-id"`
	Mount    string `yaml:"mount,omitempty"`
}

type TLS struct {
	CACertFile         string `yaml:"ca-cert-file,omitempty"`
	ClientCertFile     string `yaml:"client-cert-file,omitempty"`
	ClientKeyFile      string `yaml:"client-key-file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure-skip-verify,omitempty"`
}

type Metadata struct {
	BaseDir string `yaml:"base-dir,omitempty"`
}
