package fsmetadata

import (
	"path/filepath"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

var _ metadatadomain.MetadataService = (*FSMetadataService)(nil)
var _ metadatadomain.ResourceOperationSpecRenderer = (*FSMetadataService)(nil)

type metadataPathKind int

const (
	metadataPathResource metadataPathKind = iota
	metadataPathCollection
)

type FSMetadataService struct {
	baseDir string
}

func NewFSMetadataService(baseDir string, _ ...string) *FSMetadataService {
	return &FSMetadataService{
		baseDir: filepath.Clean(baseDir),
	}
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}

func metadataPathKindName(kind metadataPathKind) string {
	switch kind {
	case metadataPathCollection:
		return "collection"
	case metadataPathResource:
		return "resource"
	default:
		return "unknown"
	}
}
