package reconciler

import "github.com/crmarques/declarest/resource"

type Reconciler interface {
	Init() error
	GetRemoteResource(path string) (resource.Resource, error)
	GetLocalResource(path string) (resource.Resource, error)
	DeleteRemoteResource(path string) error
	DeleteLocalResource(path string) error
	SaveRemoteResource(path string, data resource.Resource) error
	CreateRemoteResource(path string, data resource.Resource) error
	UpdateRemoteResource(path string, data resource.Resource) error
	SaveLocalResource(path string, data resource.Resource) error
	SyncRemoteResource(path string) error
	SyncLocalResource(path string) error
	SyncAllResources() error
	ListLocalResourcePaths() []string
	DiffResource(path string) (resource.ResourcePatch, error)
	CheckIfResourceIsSynced(path string) (bool, error)
	GetRemoteResourcePath(path string) (string, error)
	GetLocalResourcePath(path string) (string, error)
	GetRemoteCollectionPath(path string) (string, error)
	GetLocalCollectionPath(path string) (string, error)
}
