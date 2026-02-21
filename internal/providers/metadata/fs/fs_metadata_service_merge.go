package fsmetadata

import (
	"fmt"

	metadatadomain "github.com/crmarques/declarest/metadata"
)

func validateResourceMetadata(metadata metadatadomain.ResourceMetadata) error {
	keys := sortedOperationKeys(metadata.Operations)
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return validationError(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}
	}
	return nil
}
