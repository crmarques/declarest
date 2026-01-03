package repository

import "declarest/internal/resource"

type ResourceRepositoryManager interface {
	Init() error
	GetResource(path string) (resource.Resource, error)
	CreateResource(path string, resource resource.Resource) error
	UpdateResource(path string, resource resource.Resource) error
	ApplyResource(path string, resource resource.Resource) error
	DeleteResource(path string) error
	GetResourceCollection(path string) ([]resource.Resource, error)
	ListResourcePaths() []string
	Close() error
}

type MetadataRepositoryManager interface {
	ReadMetadata(path string) (map[string]any, error)
	WriteMetadata(path string, metadata map[string]any) error
	DeleteMetadata(path string) error
}

type ResourceRepositoryMover interface {
	MoveResourceTree(fromPath, toPath string) error
}

type ResourceRepositoryRebaser interface {
	RebaseLocalFromRemote() error
}

type ResourceRepositoryPusher interface {
	PushLocalDiffsToRemote() error
}

type ResourceRepositoryForcePusher interface {
	ForcePushLocalDiffsToRemote() error
}

type ResourceRepositoryResetter interface {
	ResetLocal() error
}
