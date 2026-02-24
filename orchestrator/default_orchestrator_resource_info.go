package orchestrator

import (
	"context"
	"path"
	"strings"

	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

func (r *DefaultOrchestrator) buildResourceInfo(
	ctx context.Context,
	logicalPath string,
	value resource.Value,
) (resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	collectionPath := collectionPathFor(normalizedPath)

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return resource.Resource{}, err
	}

	normalizedPayload, err := resource.Normalize(value)
	if err != nil {
		return resource.Resource{}, err
	}

	localAlias, remoteID, err := identity.ResolveAliasAndRemoteID(normalizedPath, resolvedMetadata, normalizedPayload)
	if err != nil {
		return resource.Resource{}, err
	}

	return resource.Resource{
		LogicalPath:    normalizedPath,
		CollectionPath: collectionPath,
		LocalAlias:     localAlias,
		RemoteID:       remoteID,
		Metadata:       resolvedMetadata,
		Payload:        normalizedPayload,
	}, nil
}

func (r *DefaultOrchestrator) buildResourceInfoForRemoteRead(
	ctx context.Context,
	logicalPath string,
) (resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedMetadata, err := r.resolveMetadataForPath(ctx, normalizedPath, true)
	if err != nil {
		return resource.Resource{}, err
	}

	localAlias := path.Base(normalizedPath)
	if normalizedPath == "/" {
		localAlias = "/"
	}
	remoteID := localAlias

	payload := map[string]any{}
	if aliasAttribute := strings.TrimSpace(resolvedMetadata.AliasFromAttribute); aliasAttribute != "" {
		payload[aliasAttribute] = localAlias
	}
	if idAttribute := strings.TrimSpace(resolvedMetadata.IDFromAttribute); idAttribute != "" {
		payload[idAttribute] = remoteID
	}
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
		return resource.Resource{}, err
	}

	return resource.Resource{
		LogicalPath:    normalizedPath,
		CollectionPath: collectionPathFor(normalizedPath),
		LocalAlias:     localAlias,
		RemoteID:       remoteID,
		Metadata:       resolvedMetadata,
		Payload:        normalizedPayload,
	}, nil
}
