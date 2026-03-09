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
	applyPayloadTemplateScope(templateValue, metadata, resource.PayloadDescriptor{})

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
	resolvedResource := resource.Resource{
		LogicalPath:    input.LogicalPath,
		CollectionPath: input.CollectionPath,
		LocalAlias:     input.LocalAlias,
		RemoteID:       input.RemoteID,
		Payload:        input.Payload,
	}

	debugctx.Printf(
		ctx,
		"metadata fs render-resource start logical_path=%q operation=%q payload_type=%T",
		resolvedResource.LogicalPath,
		operation,
		resolvedResource.Payload,
	)

	targetPath, err := normalizeResolvePath(resolvedResource.LogicalPath)
	if err != nil {
		debugctx.Printf(
			ctx,
			"metadata fs render-resource invalid logical_path=%q operation=%q error=%v",
			resolvedResource.LogicalPath,
			operation,
			err,
		)
		return metadatadomain.OperationSpec{}, err
	}

	resolvedMetadata := metadatadomain.CloneResourceMetadata(input.Metadata)
	if metadataEmpty(resolvedMetadata) {
		resolvedMetadata, err = s.ResolveForPath(ctx, targetPath)
		if err != nil {
			if faults.IsCategory(err, faults.NotFoundError) {
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

	templateScope, err := buildTemplateScopeForResource(targetPath, resolvedMetadata, resolvedResource)
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
	applyPayloadTemplateScope(templateScope, resolvedMetadata, resolvedResource.PayloadDescriptor)

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
		Payload:        normalizedPayload,
	}, metadata)
}

func buildTemplateScopeForResource(
	logicalPath string,
	resolvedMetadata metadatadomain.ResourceMetadata,
	resolvedResource resource.Resource,
) (map[string]any, error) {
	normalizedPayload, err := resource.Normalize(resolvedResource.Payload)
	if err != nil {
		return nil, err
	}

	localAlias := strings.TrimSpace(resolvedResource.LocalAlias)
	remoteID := strings.TrimSpace(resolvedResource.RemoteID)
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

	collectionPath := strings.TrimSpace(resolvedResource.CollectionPath)
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
		Payload:        normalizedPayload,
	}, resolvedMetadata)
}

func metadataEmpty(value metadatadomain.ResourceMetadata) bool {
	return strings.TrimSpace(value.IDAttribute) == "" &&
		strings.TrimSpace(value.AliasAttribute) == "" &&
		strings.TrimSpace(value.CollectionPath) == "" &&
		strings.TrimSpace(value.PayloadType) == "" &&
		value.Secret == nil &&
		value.SecretAttributes == nil &&
		value.ExternalizedAttributes == nil &&
		value.Operations == nil &&
		value.Transforms == nil
}

func applyPayloadTemplateScope(
	scope map[string]any,
	metadata metadatadomain.ResourceMetadata,
	descriptor resource.PayloadDescriptor,
) {
	if scope == nil {
		return
	}

	activeDescriptor := descriptor
	if !resource.IsPayloadDescriptorExplicit(activeDescriptor) {
		if strings.TrimSpace(metadata.PayloadType) != "" {
			activeDescriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: metadata.PayloadType})
		} else {
			activeDescriptor = payloadDescriptorFromScopeValue(scope["payload"])
		}
	}
	scope["payloadType"] = activeDescriptor.PayloadType
	scope["payloadMediaType"] = activeDescriptor.MediaType
	scope["payloadExtension"] = activeDescriptor.Extension
	if _, exists := scope["contentType"]; !exists && strings.TrimSpace(activeDescriptor.MediaType) != "" {
		if _, isPayloadMap := scope["payload"].(map[string]any); !isPayloadMap {
			scope["contentType"] = activeDescriptor.MediaType
		}
	}
}

func payloadDescriptorFromScopeValue(value any) resource.PayloadDescriptor {
	switch value.(type) {
	case string:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeText})
	case resource.BinaryValue, *resource.BinaryValue:
		return resource.DefaultOctetStreamDescriptor()
	default:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
	}
}
