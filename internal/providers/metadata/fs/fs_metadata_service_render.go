package fsmetadata

import (
	"context"

	"github.com/crmarques/declarest/internal/support/identity"
	"github.com/crmarques/declarest/internal/support/templatescope"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (s *FSMetadataService) RenderOperationSpec(
	ctx context.Context,
	logicalPath string,
	operation metadatadomain.Operation,
	value any,
) (metadatadomain.OperationSpec, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	metadata, err := s.ResolveForPath(ctx, targetPath)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	templateValue, err := buildTemplateValue(targetPath, metadata, value)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}

	spec, err := metadatadomain.ResolveOperationSpecWithScope(ctx, metadata, operation, templateValue)
	if err != nil {
		return metadatadomain.OperationSpec{}, err
	}
	return spec, nil
}

func buildTemplateValue(
	logicalPath string,
	metadata metadatadomain.ResourceMetadata,
	value any,
) (map[string]any, error) {
	normalizedPayload, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	alias, remoteID, err := identity.ResolveAliasAndRemoteID(logicalPath, metadata, normalizedPayload)
	if err != nil {
		return nil, err
	}

	return templatescope.BuildOperationScope(
		logicalPath,
		collectionPathForLogicalPath(logicalPath),
		alias,
		remoteID,
		normalizedPayload,
	)
}
