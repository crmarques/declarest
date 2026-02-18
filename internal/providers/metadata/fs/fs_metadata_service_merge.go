package fsmetadata

import (
	"context"
	"fmt"

	debugctx "github.com/crmarques/declarest/internal/support/debug"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

func (s *FSMetadataService) Infer(
	ctx context.Context,
	logicalPath string,
	request metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	debugctx.Printf(
		ctx,
		"metadata fs infer start logical_path=%q apply=%t recursive=%t",
		logicalPath,
		request.Apply,
		request.Recursive,
	)

	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs infer invalid logical_path=%q error=%v",
			logicalPath,
			err,
		)
		return metadatadomain.ResourceMetadata{}, err
	}
	debugctx.Printf(ctx, "metadata fs infer normalized logical_path=%q normalized=%q", logicalPath, targetPath)

	inferred, err := metadatadomain.InferFromOpenAPI(ctx, targetPath, request)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs infer failed logical_path=%q error=%v", targetPath, err)
		return metadatadomain.ResourceMetadata{}, err
	}

	existing, found, err := s.tryReadMetadata(targetPath, metadataPathResource)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs infer read-existing failed logical_path=%q error=%v", targetPath, err)
		return metadatadomain.ResourceMetadata{}, err
	}
	if found {
		inferred = metadatadomain.MergeResourceMetadata(inferred, existing)
		debugctx.Printf(ctx, "metadata fs infer merged-existing logical_path=%q", targetPath)
	}

	if request.Apply {
		if err := s.Set(ctx, targetPath, inferred); err != nil {
			debugctx.Printf(ctx, "metadata fs infer apply failed logical_path=%q error=%v", targetPath, err)
			return metadatadomain.ResourceMetadata{}, err
		}
		debugctx.Printf(ctx, "metadata fs infer applied logical_path=%q", targetPath)
	}

	debugctx.Printf(ctx, "metadata fs infer done logical_path=%q", targetPath)
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
