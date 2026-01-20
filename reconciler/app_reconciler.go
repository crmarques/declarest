package reconciler

import (
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

type AppReconciler interface {
	Reconciler
	Close() error

	InitRepositoryLocal() error
	InitRepositoryRemoteIfEmpty() (bool, error)
	RefreshRepository() error
	UpdateRemoteRepositoryWithForce(force bool) error
	ResetRepository() error
	SetSkipRepositorySync(skip bool)
	CheckLocalRepositoryAccess() error
	CheckRemoteAccess() (bool, bool, error)
	CheckLocalRepositoryInitialized() (bool, bool, error)
	CheckRemoteSync() (bool, bool, error)
	RepositoryResourcePathsWithErrors() ([]string, error)
	RepositoryPathsInCollection(path string) ([]string, error)

	ResourceRecord(path string) (resource.ResourceRecord, error)
	MergedMetadata(path string) (resource.ResourceMetadata, error)
	ResourceMetadata(path string) (resource.ResourceMetadata, error)
	UpdateLocalMetadata(path string, update func(map[string]any) (bool, error)) error
	WriteLocalMetadata(path string, metadata map[string]any) error
	UpdateLocalResourcesForMetadata(path string) ([]LocalResourceUpdateResult, error)
	MetadataChildCollections(baseSegments []string) ([]string, error)
	OpenAPISpec() *openapi.Spec

	ListRemoteResourceEntries(path string) ([]RemoteResourceEntry, error)
	ListRemoteResourcePaths(path string) ([]string, error)
	ListRemoteResourcePathsFromLocal() ([]string, error)

	InitSecrets() error
	EnsureSecretsFile() error
	GetSecret(resourcePath string, key string) (string, error)
	SetSecret(resourcePath string, key string, value string) error
	DeleteSecret(resourcePath string, key string) error
	ListSecretKeys(resourcePath string) ([]string, error)
	ListSecretResources() ([]string, error)
	SecretPathsFor(path string) ([]string, error)
	MaskResourceSecrets(path string, res resource.Resource, store bool) (resource.Resource, error)
	ResolveResourceSecrets(path string, res resource.Resource) (resource.Resource, error)
	SaveLocalResourceWithSecrets(path string, res resource.Resource, storeSecrets bool) error
	SaveLocalCollectionItemsWithSecrets(path string, items []resource.Resource, storeSecrets bool) error
	SecretsConfigured() bool

	ManagedServerConfigured() bool
	CheckManagedServerAccess() error

	RepositoryDebugInfo() (repository.RepositoryDebugInfo, bool)
	ServerDebugInfo() (managedserver.ServerDebugInfo, bool)

	ExecuteHTTPRequest(spec *managedserver.HTTPRequestSpec, payload []byte) (*managedserver.HTTPResponse, error)
}
