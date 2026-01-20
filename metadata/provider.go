package metadata

import (
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"
)

type Provider interface {
	GetResourceRecord(path string) (resource.ResourceRecord, error)
	GetMergedMetadata(path string) (resource.ResourceMetadata, error)
}

type OpenAPISpecProvider interface {
	OpenAPISpec() *openapi.Spec
}

type ChildCollectionProvider interface {
	MetadataChildCollections(baseSegments []string) ([]string, error)
}

type RemoteRecordLoader interface {
	GetRemoteResourceWithRecord(record resource.ResourceRecord, logicalPath string, isCollection bool) (resource.Resource, error)
}
