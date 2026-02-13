package managedserver

import "github.com/crmarques/declarest/resource"

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

type AccessChecker interface {
	CheckAccess() error
}

type OpenAPISpecLoader interface {
	LoadOpenAPISpec(source string) ([]byte, error)
}

type HTTPRequestExecutor interface {
	ExecuteRequest(spec *HTTPRequestSpec, payload []byte) (*HTTPResponse, error)
}

type ServerDebugInfoProvider interface {
	DebugInfo() ServerDebugInfo
}
