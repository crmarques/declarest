package orchestrator

import (
	"context"
	"path"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	metadatavalidation "github.com/crmarques/declarest/metadata/validation"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

func (r *Orchestrator) buildResourceInfo(
	ctx context.Context,
	logicalPath string,
	content resource.Content,
) (resource.Resource, metadata.ResourceMetadata, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	collectionPath := collectionPathFor(normalizedPath)

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	expandedContent, err := r.expandExternalizedPayload(ctx, normalizedPath, resolvedMetadata, content)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	normalizedPayload, err := resource.Normalize(expandedContent.Value)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	localAlias, remoteID, err := identity.ResolveAliasAndRemoteID(normalizedPath, resolvedMetadata, normalizedPayload)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	return resource.Resource{
		LogicalPath:       normalizedPath,
		CollectionPath:    collectionPath,
		LocalAlias:        localAlias,
		RemoteID:          remoteID,
		Payload:           normalizedPayload,
		PayloadDescriptor: expandedContent.Descriptor,
	}, resolvedMetadata, nil
}

func (r *Orchestrator) buildResourceInfoForRemoteRead(
	ctx context.Context,
	logicalPath string,
) (resource.Resource, metadata.ResourceMetadata, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	localAlias := path.Base(normalizedPath)
	if normalizedPath == "/" {
		localAlias = "/"
	}
	remoteID := localAlias

	payload := map[string]any{}
	payload = metadatavalidation.MergePayloadFields(
		payload,
		metadatavalidation.DerivePathFields(resource.Resource{
			LogicalPath:    normalizedPath,
			CollectionPath: collectionPathFor(normalizedPath),
			LocalAlias:     localAlias,
			RemoteID:       remoteID,
		}, resolvedMetadata),
	).(map[string]any)
	for key, value := range templatescope.DerivePathTemplateFields(normalizedPath, resolvedMetadata) {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		if _, exists := payload[key]; exists {
			continue
		}
		payload[key] = value
	}

	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return resource.Resource{}, metadata.ResourceMetadata{}, err
	}

	descriptor := resource.PayloadDescriptor{}
	if strings.TrimSpace(resolvedMetadata.PayloadType) != "" {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resolvedMetadata.PayloadType})
	}

	return resource.Resource{
		LogicalPath:       normalizedPath,
		CollectionPath:    collectionPathFor(normalizedPath),
		LocalAlias:        localAlias,
		RemoteID:          remoteID,
		Payload:           normalizedPayload,
		PayloadDescriptor: descriptor,
	}, resolvedMetadata, nil
}
