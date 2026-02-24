package fsmetadata

import (
	"path/filepath"

	"github.com/crmarques/declarest/config"
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
	baseDir        string
	resourceFormat string
	extension      string
}

func NewFSMetadataService(baseDir string, resourceFormat string) *FSMetadataService {
	format := resourceFormat
	if format == "" {
		format = config.ResourceFormatJSON
	}

	extension := ".json"
	if format == config.ResourceFormatYAML {
		extension = ".yaml"
	}

	return &FSMetadataService{
		baseDir:        filepath.Clean(baseDir),
		resourceFormat: format,
		extension:      extension,
	}
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
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
