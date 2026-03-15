// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	if format := metadata.NormalizeResourceFormat(resolvedMetadata.Format); format != "" && !metadata.ResourceFormatAllowsMixedItems(format) {
		descriptor = resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: format})
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
