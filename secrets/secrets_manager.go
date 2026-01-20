package secrets

type SecretsManager interface {
	Init() error
	GetSecret(resourcePath string, key string) (string, error)
	SetSecret(resourcePath string, key string, value string) error
	DeleteSecret(resourcePath string, key string) error
	ListKeys(resourcePath string) ([]string, error)
	ListResources() ([]string, error)
	Close() error
}
