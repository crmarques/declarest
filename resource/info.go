package resource

import (
	"github.com/crmarques/declarest/core"
	"github.com/crmarques/declarest/metadata"
)

type Info struct {
	LogicalPath        string
	CollectionPath     string
	LocalAlias         string
	RemoteID           string
	ResolvedRemotePath string
	Metadata           metadata.ResourceMetadata
	Payload            core.Resource
}
