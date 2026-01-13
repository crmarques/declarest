package context

import (
	"declarest/internal/managedserver"
	"declarest/internal/repository"
	"declarest/internal/secrets"
)

type ContextConfig struct {
	ManagedServer *ManagedServerConfig          `mapstructure:"managed_server" yaml:"managed_server,omitempty" json:"managed_server,omitempty"`
	Repository    *RepositoryConfig             `mapstructure:"repository" yaml:"repository,omitempty" json:"repository,omitempty"`
	SecretManager *secrets.SecretsManagerConfig `mapstructure:"secret_store" yaml:"secret_store,omitempty" json:"secret_store,omitempty"`
	Metadata      *MetadataConfig               `mapstructure:"metadata" yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type ManagedServerConfig struct {
	HTTP *managedserver.HTTPResourceServerConfig `mapstructure:"http" yaml:"http,omitempty" json:"http,omitempty"`
}

type RepositoryConfig struct {
	ResourceFormat string                                         `mapstructure:"resource_format" yaml:"resource_format,omitempty" json:"resource_format,omitempty"`
	Git            *repository.GitResourceRepositoryConfig        `mapstructure:"git" yaml:"git,omitempty" json:"git,omitempty"`
	Filesystem     *repository.FileSystemResourceRepositoryConfig `mapstructure:"filesystem" yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
}

type MetadataConfig struct {
	BaseDir string `mapstructure:"base_dir" yaml:"base_dir,omitempty" json:"base_dir,omitempty"`
}
