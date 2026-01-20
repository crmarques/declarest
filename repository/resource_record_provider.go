package repository

import "github.com/crmarques/declarest/resource"

type ResourceRecordProvider interface {
	GetResourceRecord(path string) (resource.ResourceRecord, error)
	GetMergedMetadata(path string) (resource.ResourceMetadata, error)
}
