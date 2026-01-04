package secrets

type SecretsManagerConfig struct {
	File  *FileSecretsManagerConfig  `mapstructure:"file" yaml:"file,omitempty" json:"file,omitempty"`
	Vault *VaultSecretsManagerConfig `mapstructure:"vault" yaml:"vault,omitempty" json:"vault,omitempty"`
}

type FileSecretsManagerConfig struct {
	Path           string                       `mapstructure:"path" yaml:"path,omitempty" json:"path,omitempty"`
	Key            string                       `mapstructure:"key" yaml:"key,omitempty" json:"key,omitempty"`
	KeyFile        string                       `mapstructure:"key_file" yaml:"key_file,omitempty" json:"key_file,omitempty"`
	Passphrase     string                       `mapstructure:"passphrase" yaml:"passphrase,omitempty" json:"passphrase,omitempty"`
	PassphraseFile string                       `mapstructure:"passphrase_file" yaml:"passphrase_file,omitempty" json:"passphrase_file,omitempty"`
	KDF            *FileSecretsManagerKDFConfig `mapstructure:"kdf" yaml:"kdf,omitempty" json:"kdf,omitempty"`
}

type FileSecretsManagerKDFConfig struct {
	Time    uint32 `mapstructure:"time" yaml:"time,omitempty" json:"time,omitempty"`
	Memory  uint32 `mapstructure:"memory" yaml:"memory,omitempty" json:"memory,omitempty"`
	Threads uint8  `mapstructure:"threads" yaml:"threads,omitempty" json:"threads,omitempty"`
}

type VaultSecretsManagerConfig struct {
	Address    string                         `mapstructure:"address" yaml:"address,omitempty" json:"address,omitempty"`
	Mount      string                         `mapstructure:"mount" yaml:"mount,omitempty" json:"mount,omitempty"`
	PathPrefix string                         `mapstructure:"path_prefix" yaml:"path_prefix,omitempty" json:"path_prefix,omitempty"`
	KVVersion  int                            `mapstructure:"kv_version" yaml:"kv_version,omitempty" json:"kv_version,omitempty"`
	Auth       *VaultSecretsManagerAuthConfig `mapstructure:"auth" yaml:"auth,omitempty" json:"auth,omitempty"`
	TLS        *VaultSecretsManagerTLSConfig  `mapstructure:"tls" yaml:"tls,omitempty" json:"tls,omitempty"`
}

type VaultSecretsManagerAuthConfig struct {
	Token    string                                 `mapstructure:"token" yaml:"token,omitempty" json:"token,omitempty"`
	Password *VaultSecretsManagerPasswordAuthConfig `mapstructure:"password" yaml:"password,omitempty" json:"password,omitempty"`
	AppRole  *VaultSecretsManagerAppRoleAuthConfig  `mapstructure:"approle" yaml:"approle,omitempty" json:"approle,omitempty"`
}

type VaultSecretsManagerPasswordAuthConfig struct {
	Username string `mapstructure:"username" yaml:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password" yaml:"password,omitempty" json:"password,omitempty"`
	Mount    string `mapstructure:"mount" yaml:"mount,omitempty" json:"mount,omitempty"`
}

type VaultSecretsManagerAppRoleAuthConfig struct {
	RoleID   string `mapstructure:"role_id" yaml:"role_id,omitempty" json:"role_id,omitempty"`
	SecretID string `mapstructure:"secret_id" yaml:"secret_id,omitempty" json:"secret_id,omitempty"`
	Mount    string `mapstructure:"mount" yaml:"mount,omitempty" json:"mount,omitempty"`
}

type VaultSecretsManagerTLSConfig struct {
	CACertFile         string `mapstructure:"ca_cert_file" yaml:"ca_cert_file,omitempty" json:"ca_cert_file,omitempty"`
	ClientCertFile     string `mapstructure:"client_cert_file" yaml:"client_cert_file,omitempty" json:"client_cert_file,omitempty"`
	ClientKeyFile      string `mapstructure:"client_key_file" yaml:"client_key_file,omitempty" json:"client_key_file,omitempty"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
}
