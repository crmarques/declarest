package managedserver

import "declarest/internal/resource"

type ResourceServerManager interface {
	Init() error
	GetResource(spec RequestSpec) (resource.Resource, error)
	GetResourceCollection(spec RequestSpec) ([]resource.Resource, error)
	CreateResource(data resource.Resource, spec RequestSpec) error
	UpdateResource(data resource.Resource, spec RequestSpec) error
	DeleteResource(spec RequestSpec) error
	ResourceExists(spec RequestSpec) (bool, error)
	Close() error
}
