package repository

import "declarest/internal/resource"

type ResourceRecordProvider interface {
	GetResourceRecord(path string) (resource.ResourceRecord, error)
	GetMergedMetadata(path string) (resource.ResourceMetadata, error)
}
