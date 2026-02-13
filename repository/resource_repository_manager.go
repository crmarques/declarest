package repository

import "github.com/crmarques/declarest/resource"

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

type ResourceRepositoryPathLister interface {
	ListResourcePathsWithErrors() ([]string, error)
}

type ResourceRepositoryBatcher interface {
	RunBatch(fn func() error) error
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

type RepositorySyncer interface {
	SyncLocalFromRemoteIfConfigured() error
}

type LocalRepositoryInitializer interface {
	InitLocalRepository() error
}

type RemoteRepositoryInitializer interface {
	InitRemoteIfEmpty() (bool, error)
}

type RemoteAccessChecker interface {
	CheckRemoteAccess() (bool, error)
}

type LocalRepositoryStateChecker interface {
	IsLocalRepositoryInitialized() (bool, error)
}

type RemoteSyncChecker interface {
	CheckRemoteSync() (bool, bool, error)
}

type MetadataBaseDirSetter interface {
	SetMetadataBaseDir(dir string)
}

type GitConfigSetter interface {
	SetConfig(cfg *GitResourceRepositoryConfig)
}

type ResourceFormatSetter interface {
	SetResourceFormat(format ResourceFormat)
}

type RepositoryDebugInfoProvider interface {
	DebugInfo() RepositoryDebugInfo
}
