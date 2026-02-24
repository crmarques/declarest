package resource

import (
	"github.com/crmarques/declarest/metadata"
)

type Value = any

type Resource struct {
	LogicalPath        string
	CollectionPath     string
	LocalAlias         string
	RemoteID           string
	ResolvedRemotePath string
	Metadata           metadata.ResourceMetadata
	Payload            Value
}

type DiffEntry struct {
	ResourcePath string
	Path         string
	Operation    string
	Local        Value
	Remote       Value
}
