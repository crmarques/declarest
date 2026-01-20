package repository

import "gopkg.in/yaml.v3"

type GitResourceRepositoryConfig struct {
	Local  *GitResourceRepositoryLocalConfig  `mapstructure:"local" yaml:"local,omitempty" json:"local,omitempty"`
	Remote *GitResourceRepositoryRemoteConfig `mapstructure:"remote" yaml:"remote,omitempty" json:"remote,omitempty"`
}

type GitResourceRepositoryLocalConfig struct {
	BaseDir string `mapstructure:"base_dir" yaml:"base_dir,omitempty" json:"base_dir,omitempty"`
}

type GitResourceRepositoryRemoteConfig struct {
	URL      string                                 `mapstructure:"url" yaml:"url,omitempty" json:"url,omitempty"`
	Branch   string                                 `mapstructure:"branch" yaml:"branch,omitempty" json:"branch,omitempty"`
	Provider string                                 `mapstructure:"provider" yaml:"provider,omitempty" json:"provider,omitempty"`
	AutoSync *bool                                  `mapstructure:"auto_sync" yaml:"auto_sync,omitempty" json:"auto_sync,omitempty"`
	Auth     *GitResourceRepositoryRemoteAuthConfig `mapstructure:"auth" yaml:"auth,omitempty" json:"auth,omitempty"`
	TLS      *GitResourceRepositoryRemoteTLSConfig  `mapstructure:"tls" yaml:"tls,omitempty" json:"tls,omitempty"`
}

func (c *GitResourceRepositoryRemoteConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw GitResourceRepositoryRemoteConfig
	var aux struct {
		raw            `yaml:",inline"`
		AutoSyncCompat *bool `yaml:"auto-sync"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}
	*c = GitResourceRepositoryRemoteConfig(aux.raw)
	if c.AutoSync == nil && aux.AutoSyncCompat != nil {
		c.AutoSync = aux.AutoSyncCompat
	}
	return nil
}

type GitResourceRepositoryRemoteAuthConfig struct {
	BasicAuth *GitResourceRepositoryBasicAuthConfig `mapstructure:"basic_auth" yaml:"basic_auth,omitempty" json:"basic_auth,omitempty"`
	SSH       *GitResourceRepositorySSHAuthConfig   `mapstructure:"ssh" yaml:"ssh,omitempty" json:"ssh,omitempty"`
	AccessKey *GitResourceRepositoryAccessKeyConfig `mapstructure:"access_key" yaml:"access_key,omitempty" json:"access_key,omitempty"`
}

type GitResourceRepositoryBasicAuthConfig struct {
	Username string `mapstructure:"username" yaml:"username,omitempty" json:"username,omitempty"`
	Password string `mapstructure:"password" yaml:"password,omitempty" json:"password,omitempty"`
}

type GitResourceRepositorySSHAuthConfig struct {
	User                  string `mapstructure:"user" yaml:"user,omitempty" json:"user,omitempty"`
	PrivateKeyFile        string `mapstructure:"private_key_file" yaml:"private_key_file,omitempty" json:"private_key_file,omitempty"`
	Passphrase            string `mapstructure:"passphrase" yaml:"passphrase,omitempty" json:"passphrase,omitempty"`
	KnownHostsFile        string `mapstructure:"known_hosts_file" yaml:"known_hosts_file,omitempty" json:"known_hosts_file,omitempty"`
	InsecureIgnoreHostKey bool   `mapstructure:"insecure_ignore_host_key" yaml:"insecure_ignore_host_key,omitempty" json:"insecure_ignore_host_key,omitempty"`
}

type GitResourceRepositoryAccessKeyConfig struct {
	Token string `mapstructure:"token" yaml:"token,omitempty" json:"token,omitempty"`
}

type GitResourceRepositoryRemoteTLSConfig struct {
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify,omitempty" json:"insecure_skip_verify,omitempty"`
}
