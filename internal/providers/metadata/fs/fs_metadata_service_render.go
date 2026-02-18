package fsmetadata

import (
	"context"

	debugctx "github.com/crmarques/declarest/internal/support/debug"
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
	debugctx.Printf(
		ctx,
		"metadata fs render start logical_path=%q operation=%q value_type=%T",
		logicalPath,
		operation,
		value,
	)

	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render invalid logical_path=%q operation=%q error=%v",
			logicalPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs render normalized logical_path=%q normalized=%q operation=%q",
		logicalPath,
		targetPath,
		operation,
	)

	metadata, err := s.ResolveForPath(ctx, targetPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render resolve failed logical_path=%q operation=%q error=%v",
			targetPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	templateValue, err := buildTemplateValue(targetPath, metadata, value)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render template-value failed logical_path=%q operation=%q error=%v",
			targetPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	spec, err := metadatadomain.ResolveOperationSpecWithScope(ctx, metadata, operation, templateValue)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render failed logical_path=%q operation=%q error=%v",
			targetPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs render done logical_path=%q operation=%q resolved_path=%q",
		logicalPath,
		operation,
		spec.Path,
	)
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
