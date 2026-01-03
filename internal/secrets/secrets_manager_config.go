package secrets

type SecretsManagerConfig struct {
	File *FileSecretsManagerConfig `mapstructure:"file" yaml:"file,omitempty" json:"file,omitempty"`
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
