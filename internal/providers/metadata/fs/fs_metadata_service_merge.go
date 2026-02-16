package fsmetadata

import (
	"context"
	"fmt"

	metadatadomain "github.com/crmarques/declarest/metadata"
)

func (s *FSMetadataService) Infer(
	ctx context.Context,
	logicalPath string,
	request metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	inferred, err := metadatadomain.InferFromOpenAPI(ctx, targetPath, request)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	existing, found, err := s.tryReadMetadata(targetPath, metadataPathResource)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	if found {
		inferred = metadatadomain.MergeResourceMetadata(inferred, existing)
	}

	if request.Apply {
		if err := s.Set(ctx, targetPath, inferred); err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
	}

	return inferred, nil
}

func validateResourceMetadata(metadata metadatadomain.ResourceMetadata) error {
	keys := sortedOperationKeys(metadata.Operations)
	for _, key := range keys {
		if !metadatadomain.Operation(key).IsValid() {
			return validationError(fmt.Sprintf("unsupported metadata operation %q", key), nil)
		}
	}
	return nil
}
