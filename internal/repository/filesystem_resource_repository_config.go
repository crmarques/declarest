package repository

type FileSystemResourceRepositoryConfig struct {
	BaseDir string `mapstructure:"base_dir" yaml:"base_dir,omitempty" json:"base_dir,omitempty"`
}
