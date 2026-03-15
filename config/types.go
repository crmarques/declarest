package config

type ContextSelection struct {
	Name      string
	Overrides map[string]string
}

const (
	ContextFileEnvVar         = "DECLAREST_CONTEXTS_FILE"
	DefaultContextCatalogPath = "~/.declarest/configs/contexts.yaml"
	GitProviderGitHub         = "github"
	OAuthClientCreds          = "client_credentials"
)

type ContextCatalog struct {
	Contexts       []Context `json:"contexts" yaml:"contexts"`
	CurrentContext string    `json:"currentContext" yaml:"currentContext"`
	DefaultEditor  string    `json:"defaultEditor,omitempty" yaml:"defaultEditor,omitempty"`
}

type Context struct {
	Name          string            `json:"name" yaml:"name"`
	Repository    Repository        `json:"repository" yaml:"repository"`
	ManagedServer *ManagedServer    `json:"managedServer,omitempty" yaml:"managedServer,omitempty"`
	SecretStore   *SecretStore      `json:"secretStore,omitempty" yaml:"secretStore,omitempty"`
	Metadata      Metadata          `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Preferences   map[string]string `json:"preferences,omitempty" yaml:"preferences,omitempty"`
}

type Repository struct {
	Git        *GitRepository        `json:"git,omitempty" yaml:"git,omitempty"`
	Filesystem *FilesystemRepository `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
}

type GitRepository struct {
	Local  GitLocal   `json:"local" yaml:"local"`
	Remote *GitRemote `json:"remote,omitempty" yaml:"remote,omitempty"`
}

type GitLocal struct {
	BaseDir  string `json:"baseDir" yaml:"baseDir"`
	AutoInit *bool  `json:"autoInit,omitempty" yaml:"autoInit,omitempty"`
}

func (g GitLocal) AutoInitEnabled() bool {
	if g.AutoInit == nil {
		return true
	}
	return *g.AutoInit
}

type GitRemote struct {
	URL      string     `json:"url" yaml:"url"`
	Branch   string     `json:"branch,omitempty" yaml:"branch,omitempty"`
	Provider string     `json:"provider,omitempty" yaml:"provider,omitempty"`
	AutoSync *bool      `json:"autoSync,omitempty" yaml:"autoSync,omitempty"`
	Auth     *GitAuth   `json:"auth,omitempty" yaml:"auth,omitempty"`
	TLS      *TLS       `json:"tls,omitempty" yaml:"tls,omitempty"`
	Proxy    *HTTPProxy `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

func (g GitRemote) AutoSyncEnabled() bool {
	if g.AutoSync == nil {
		return true
	}
	return *g.AutoSync
}

type GitAuth struct {
	BasicAuth *BasicAuth     `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	SSH       *SSHAuth       `json:"ssh,omitempty" yaml:"ssh,omitempty"`
	AccessKey *AccessKeyAuth `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	Prompt    *PromptAuth    `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

type FilesystemRepository struct {
	BaseDir string `json:"baseDir" yaml:"baseDir"`
}

type ManagedServer struct {
	HTTP *HTTPServer `json:"http,omitempty" yaml:"http,omitempty"`
}

type HTTPServer struct {
	BaseURL           string                 `json:"baseURL" yaml:"baseURL"`
	HealthCheck       string                 `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
	OpenAPI           string                 `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	DefaultHeaders    map[string]string      `json:"defaultHeaders,omitempty" yaml:"defaultHeaders,omitempty"`
	Auth              *HTTPAuth              `json:"auth,omitempty" yaml:"auth,omitempty"`
	Proxy             *HTTPProxy             `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	TLS               *TLS                   `json:"tls,omitempty" yaml:"tls,omitempty"`
	RequestThrottling *HTTPRequestThrottling `json:"requestThrottling,omitempty" yaml:"requestThrottling,omitempty"`
}

type HTTPRequestThrottling struct {
	MaxConcurrentRequests int     `json:"maxConcurrentRequests,omitempty" yaml:"maxConcurrentRequests,omitempty"`
	QueueSize             int     `json:"queueSize,omitempty" yaml:"queueSize,omitempty"`
	RequestsPerSecond     float64 `json:"requestsPerSecond,omitempty" yaml:"requestsPerSecond,omitempty"`
	Burst                 int     `json:"burst,omitempty" yaml:"burst,omitempty"`
	ScopeKey              string  `json:"-" yaml:"-"`
}

type HTTPProxy struct {
	HTTPURL  string     `json:"httpURL,omitempty" yaml:"httpURL,omitempty"`
	HTTPSURL string     `json:"httpsURL,omitempty" yaml:"httpsURL,omitempty"`
	NoProxy  string     `json:"noProxy,omitempty" yaml:"noProxy,omitempty"`
	Auth     *ProxyAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
}

type ProxyAuth struct {
	Username string      `json:"username" yaml:"username"`
	Password string      `json:"password" yaml:"password"`
	Prompt   *PromptAuth `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

type HTTPAuth struct {
	OAuth2        *OAuth2           `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`
	BasicAuth     *BasicAuth        `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	CustomHeaders []HeaderTokenAuth `json:"customHeaders,omitempty" yaml:"customHeaders,omitempty"`
	Prompt        *PromptAuth       `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

type PromptAuth struct {
	KeepCredentialsForSession bool `json:"keepCredentialsForSession,omitempty" yaml:"keepCredentialsForSession,omitempty"`
}

type OAuth2 struct {
	TokenURL     string `json:"tokenURL" yaml:"tokenURL"`
	GrantType    string `json:"grantType" yaml:"grantType"`
	ClientID     string `json:"clientID" yaml:"clientID"`
	ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
	Username     string `json:"username,omitempty" yaml:"username,omitempty"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	Scope        string `json:"scope,omitempty" yaml:"scope,omitempty"`
	Audience     string `json:"audience,omitempty" yaml:"audience,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

type HeaderTokenAuth struct {
	Header string `json:"header" yaml:"header"`
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Value  string `json:"value" yaml:"value"`
}

type SSHAuth struct {
	User                  string `json:"user" yaml:"user"`
	PrivateKeyFile        string `json:"privateKeyFile" yaml:"privateKeyFile"`
	Passphrase            string `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
	KnownHostsFile        string `json:"knownHostsFile,omitempty" yaml:"knownHostsFile,omitempty"`
	InsecureIgnoreHostKey bool   `json:"insecureIgnoreHostKey,omitempty" yaml:"insecureIgnoreHostKey,omitempty"`
}

type AccessKeyAuth struct {
	Token string `json:"token" yaml:"token"`
}

type SecretStore struct {
	File  *FileSecretStore  `json:"file,omitempty" yaml:"file,omitempty"`
	Vault *VaultSecretStore `json:"vault,omitempty" yaml:"vault,omitempty"`
}

type FileSecretStore struct {
	Path           string `json:"path" yaml:"path"`
	Key            string `json:"key,omitempty" yaml:"key,omitempty"`
	KeyFile        string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	Passphrase     string `json:"passphrase,omitempty" yaml:"passphrase,omitempty"`
	PassphraseFile string `json:"passphraseFile,omitempty" yaml:"passphraseFile,omitempty"`
	KDF            *KDF   `json:"kdf,omitempty" yaml:"kdf,omitempty"`
}

type KDF struct {
	Time    int `json:"time,omitempty" yaml:"time,omitempty"`
	Memory  int `json:"memory,omitempty" yaml:"memory,omitempty"`
	Threads int `json:"threads,omitempty" yaml:"threads,omitempty"`
}

type VaultSecretStore struct {
	Address    string     `json:"address" yaml:"address"`
	Mount      string     `json:"mount,omitempty" yaml:"mount,omitempty"`
	PathPrefix string     `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`
	KVVersion  int        `json:"kvVersion,omitempty" yaml:"kvVersion,omitempty"`
	Auth       *VaultAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
	TLS        *TLS       `json:"tls,omitempty" yaml:"tls,omitempty"`
	Proxy      *HTTPProxy `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

type VaultAuth struct {
	Token    string                 `json:"token,omitempty" yaml:"token,omitempty"`
	Password *VaultUserPasswordAuth `json:"password,omitempty" yaml:"password,omitempty"`
	AppRole  *VaultAppRoleAuth      `json:"appRole,omitempty" yaml:"appRole,omitempty"`
	Prompt   *VaultPromptAuth       `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

type VaultUserPasswordAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Mount    string `json:"mount,omitempty" yaml:"mount,omitempty"`
}

type VaultPromptAuth struct {
	KeepCredentialsForSession bool   `json:"keepCredentialsForSession,omitempty" yaml:"keepCredentialsForSession,omitempty"`
	Mount                     string `json:"mount,omitempty" yaml:"mount,omitempty"`
}

type VaultAppRoleAuth struct {
	RoleID   string `json:"roleID" yaml:"roleID"`
	SecretID string `json:"secretID" yaml:"secretID"`
	Mount    string `json:"mount,omitempty" yaml:"mount,omitempty"`
}

type TLS struct {
	CACertFile         string `json:"caCertFile,omitempty" yaml:"caCertFile,omitempty"`
	ClientCertFile     string `json:"clientCertFile,omitempty" yaml:"clientCertFile,omitempty"`
	ClientKeyFile      string `json:"clientKeyFile,omitempty" yaml:"clientKeyFile,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

type Metadata struct {
	BaseDir    string     `json:"baseDir,omitempty" yaml:"baseDir,omitempty"`
	Bundle     string     `json:"bundle,omitempty" yaml:"bundle,omitempty"`
	BundleFile string     `json:"bundleFile,omitempty" yaml:"bundleFile,omitempty"`
	Proxy      *HTTPProxy `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}
