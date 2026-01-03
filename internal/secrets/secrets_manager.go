package secrets

type SecretsManager interface {
	Init() error
	GetSecret(resourcePath string, key string) (string, error)
	CreateSecret(resourcePath string, key string, value string) error
	UpdateSecret(resourcePath string, key string, value string) error
	DeleteSecret(resourcePath string, key string, value string) error
	ListKeys(resourcePath string) []string
	ListResources() ([]string, error)
	Close() error
}
