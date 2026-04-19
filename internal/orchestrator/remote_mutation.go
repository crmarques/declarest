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
	"fmt"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

func (r *Orchestrator) executeRemoteMutation(
	ctx context.Context,
	resolvedResource resource.Resource,
	md metadata.ResourceMetadata,
	operation metadata.Operation,
) (resource.Resource, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return resource.Resource{}, err
	}

	var remotePayload resource.Content
	switch operation {
	case metadata.OperationCreate:
		remotePayload, err = serverManager.Create(ctx, resolvedResource, md)
	case metadata.OperationUpdate:
		remotePayload, err = serverManager.Update(ctx, resolvedResource, md)
	default:
		return resource.Resource{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported remote mutation operation %q", operation),
			nil,
		)
	}
	if err != nil {
		return resource.Resource{}, err
	}

	payload := resolvedResource.Payload
	descriptor := resolvedResource.PayloadDescriptor
	if remotePayload.Value != nil {
		payload = remotePayload.Value
		descriptor = remotePayload.Descriptor
	}
	normalizedPayload, err := resource.Normalize(payload)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedResource.Payload = normalizedPayload
	resolvedResource.PayloadDescriptor = descriptor
	return resolvedResource, nil
}

func (r *Orchestrator) resolvePayloadForRemote(
	ctx context.Context,
	logicalPath string,
	content resource.Content,
) (resource.Content, error) {
	if content.Value == nil {
		return content, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Content{}, err
	}

	var getSecret func(string) (string, error)
	if r != nil && r.secrets != nil {
		getSecret = func(key string) (string, error) {
			return r.secrets.Get(ctx, key)
		}
	}

	resolved, err := secrets.ResolvePayloadDirectivesForResource(
		content.Value,
		normalizedPath,
		content.Descriptor,
		getSecret,
	)
	if err != nil {
		return resource.Content{}, err
	}
	return resource.Content{
		Value:      resolved,
		Descriptor: content.Descriptor,
	}, nil
}
