package fsmetadata

import (
	"context"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
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
	templateValue["resourceFormat"] = metadatadomain.NormalizeResourceFormat(s.resourceFormat)

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

func (s *FSMetadataService) RenderOperationSpecForResource(
	ctx context.Context,
	input metadatadomain.ResourceOperationSpecInput,
	operation metadatadomain.Operation,
) (metadatadomain.OperationSpec, error) {
	resourceInfo := resource.Resource{
		LogicalPath:    input.LogicalPath,
		CollectionPath: input.CollectionPath,
		LocalAlias:     input.LocalAlias,
		RemoteID:       input.RemoteID,
		Metadata:       input.Metadata,
		Payload:        input.Payload,
	}

	debugctx.Printf(
		ctx,
		"metadata fs render-resource start logical_path=%q operation=%q payload_type=%T",
		resourceInfo.LogicalPath,
		operation,
		resourceInfo.Payload,
	)

	targetPath, err := normalizeResolvePath(resourceInfo.LogicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource invalid logical_path=%q operation=%q error=%v",
			resourceInfo.LogicalPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	resolvedMetadata := metadatadomain.CloneResourceMetadata(resourceInfo.Metadata)
	if metadataEmpty(resolvedMetadata) {
		resolvedMetadata, err = s.ResolveForPath(ctx, targetPath)
		if err != nil {
			if isTypedCategory(err, faults.NotFoundError) {
				resolvedMetadata = metadatadomain.ResourceMetadata{}
			} else {
				debugctx.Printf(
					ctx,
					"metadata fs render-resource resolve failed logical_path=%q operation=%q error=%v",
					targetPath,
					operation,
					err,
				)
				return metadatadomain.OperationSpec{}, err
			}
		}
	}

	templateScope, err := buildTemplateScopeForResource(targetPath, resolvedMetadata, resourceInfo)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource template-scope failed logical_path=%q operation=%q error=%v",
			targetPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}
	templateScope["resourceFormat"] = metadatadomain.NormalizeResourceFormat(s.resourceFormat)

	spec, err := metadatadomain.ResolveOperationSpecWithScope(ctx, resolvedMetadata, operation, templateScope)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource failed logical_path=%q operation=%q error=%v",
			targetPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	debugctx.Printf(
		ctx,
		"metadata fs render-resource done logical_path=%q operation=%q resolved_path=%q",
		targetPath,
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

	return templatescope.BuildResourceScope(resource.Resource{
		LogicalPath:    logicalPath,
		CollectionPath: collectionPathForLogicalPath(logicalPath),
		LocalAlias:     alias,
		RemoteID:       remoteID,
		Metadata:       metadata,
		Payload:        normalizedPayload,
	})
}

func buildTemplateScopeForResource(
	logicalPath string,
	resolvedMetadata metadatadomain.ResourceMetadata,
	resourceInfo resource.Resource,
) (map[string]any, error) {
	normalizedPayload, err := resource.Normalize(resourceInfo.Payload)
	if err != nil {
		return nil, err
	}

	localAlias := strings.TrimSpace(resourceInfo.LocalAlias)
	remoteID := strings.TrimSpace(resourceInfo.RemoteID)
	if localAlias == "" || remoteID == "" {
		derivedAlias, derivedRemoteID, identityErr := identity.ResolveAliasAndRemoteID(
			logicalPath,
			resolvedMetadata,
			normalizedPayload,
		)
		if identityErr != nil {
			return nil, identityErr
		}
		if localAlias == "" {
			localAlias = derivedAlias
		}
		if remoteID == "" {
			remoteID = derivedRemoteID
		}
	}

	collectionPath := strings.TrimSpace(resourceInfo.CollectionPath)
	if collectionPath == "" {
		collectionPath = collectionPathForLogicalPath(logicalPath)
	} else {
		collectionPath, err = resource.NormalizeLogicalPath(collectionPath)
		if err != nil {
			return nil, err
		}
	}

	return templatescope.BuildResourceScope(resource.Resource{
		LogicalPath:    logicalPath,
		CollectionPath: collectionPath,
		LocalAlias:     localAlias,
		RemoteID:       remoteID,
		Metadata:       resolvedMetadata,
		Payload:        normalizedPayload,
	})
}

func metadataEmpty(value metadatadomain.ResourceMetadata) bool {
	return strings.TrimSpace(value.IDFromAttribute) == "" &&
		strings.TrimSpace(value.AliasFromAttribute) == "" &&
		strings.TrimSpace(value.CollectionPath) == "" &&
		value.SecretsFromAttributes == nil &&
		value.Operations == nil &&
		value.Filter == nil &&
		value.Suppress == nil &&
		strings.TrimSpace(value.JQ) == ""
}

func isTypedCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}
